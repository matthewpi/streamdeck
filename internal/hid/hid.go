//
// Copyright (c) 2023 Matthew Penner
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//

package hid

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

var ErrDeviceAlreadyConnected = errors.New("hid: device already connected")

type DeviceInfo struct {
	VendorID  uint16
	ProductID uint16
	Revision  uint16

	SubClass uint8
	Protocol uint8

	Interface uint8
	Bus       int
	Device    int
}

type USB struct {
	info DeviceInfo
	path string

	fMx sync.RWMutex
	f   *os.File

	endpointIn  uint8
	endpointOut uint8

	inputPacketSize  uint16
	outputPacketSize uint16
}

// Open opens the USB HID device.
func (u *USB) Open(ctx context.Context) error {
	u.fMx.Lock()
	if u.f != nil {
		u.fMx.Unlock()
		return ErrDeviceAlreadyConnected
	}

	f, err := os.OpenFile(u.path, os.O_RDWR, 0o644)
	if err != nil {
		u.fMx.Unlock()
		return err
	}
	u.f = f
	u.fMx.Unlock()
	return u.unsafeClaim(ctx)
}

// Close closes the device.
func (u *USB) Close(ctx context.Context) error {
	u.fMx.Lock()
	defer u.fMx.Unlock()
	if u.f == nil {
		return nil
	}

	if err := u.unsafeRelease(ctx); err != nil {
		_ = u.f.Close()
		u.f = nil
		return err
	}
	if err := u.f.Close(); err != nil {
		u.f = nil
		return err
	}
	u.f = nil
	return nil
}

// Info returns information about the device.
func (u *USB) Info() DeviceInfo {
	return u.info
}

func (u *USB) Read(ctx context.Context, v []byte, t time.Duration) (int, error) {
	n, err := u.intr(ctx, u.endpointIn, v, t)
	if err == nil {
		return n, nil
	} else {
		return 0, err
	}
}

func (u *USB) Write(ctx context.Context, v []byte) (int, error) {
	if u.endpointOut > 0 {
		return u.intr(ctx, u.endpointOut, v, 1000)
	}
	return u.ctrl(ctx, 0x21, 0x09, 2<<8+0, int(u.info.Interface), v, time.Duration(len(v))*time.Millisecond)
}

func (u *USB) GetFeatureReport(ctx context.Context, v []byte) (int, error) {
	// 10100001, GET_REPORT, type*256+id, intf, len, data
	return u.ctrl(ctx, 0xa1, 0x01, (3<<8)+int(v[0]), int(u.info.Interface), v, 0)
}

func (u *USB) SendFeatureReport(ctx context.Context, v []byte) (int, error) {
	// 00100001, SET_REPORT, type*256+id, intf, len, data
	return u.ctrl(ctx, 0x21, 0x09, (3<<8)+int(v[0]), int(u.info.Interface), v, 0)
}

func (u *USB) unsafeClaim(ctx context.Context) error {
	s := &usbFSIoctl{
		Interface: uint32(u.info.Interface),
		IoctlCode: USBDevFSDisconnect,
		Data:      0,
	}
	if r, err := u.unsafeIoctl(ctx, USBDevFSIoctl, uintptr(unsafe.Pointer(s))); r == -1 {
		return err
	}
	if r, err := u.unsafeIoctl(ctx, USBDevFSClaim, uintptr(unsafe.Pointer(&u.info.Interface))); r == -1 {
		return err
	}
	return nil
}

func (u *USB) unsafeRelease(ctx context.Context) error {
	if r, err := u.unsafeIoctl(ctx, USBDevFSRelease, uintptr(unsafe.Pointer(&u.info.Interface))); r == -1 {
		return err
	}
	s := &usbFSIoctl{
		Interface: uint32(u.info.Interface),
		IoctlCode: USBDevFSConnect,
		Data:      0,
	}
	if r, err := u.unsafeIoctl(ctx, USBDevFSIoctl, uintptr(unsafe.Pointer(s))); r == -1 {
		return err
	}
	return nil
}

func (u *USB) ctrl(ctx context.Context, rtype, req, val, index int, v []byte, t time.Duration) (int, error) {
	s := &usbFSCtrl{
		ReqType: uint8(rtype),
		Req:     uint8(req),
		Value:   uint16(val),
		Index:   uint16(index),
		Len:     uint16(len(v)),
		Data:    slicePtr(v),
	}
	if t != 0 {
		s.Timeout = uint32(t.Milliseconds())
	}
	if r, err := u.ioctl(ctx, USBDevFSControl, uintptr(unsafe.Pointer(s))); r == -1 {
		return -1, err
	} else {
		return r, nil
	}
}

func (u *USB) intr(ctx context.Context, endpoint uint8, v []byte, t time.Duration) (int, error) {
	s := &usbFSBulk{
		Endpoint: uint32(endpoint),
		Len:      uint32(len(v)),
		Data:     slicePtr(v),
	}
	if t != 0 {
		s.Timeout = uint32(t.Milliseconds())
	}
	if r, err := u.ioctl(ctx, USBDevFSBulk, uintptr(unsafe.Pointer(s))); r == -1 {
		return -1, err
	} else {
		return r, nil
	}
}

// unsafeIoctl is like ioctl but is unsafe as it doesn't lock `u.f` before
// reading its file descriptor.
func (u *USB) unsafeIoctl(ctx context.Context, req uint32, v uintptr) (int, error) {
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
		r, r2, err := unix.Syscall(
			unix.SYS_IOCTL,
			u.f.Fd(),
			uintptr(req),
			v,
		)
		if err != 0 {
			fmt.Printf("r=%d r2=%d errno=%d err=%v\n", r, r2, uintptr(err), err)
		}
		return int(r), err
	}
}

func (u *USB) ioctl(ctx context.Context, req uint32, v uintptr) (int, error) {
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
		u.fMx.RLock()
		fd := u.f.Fd()
		u.fMx.RUnlock()
		r, r2, err := unix.Syscall(
			unix.SYS_IOCTL,
			fd,
			uintptr(req),
			v,
		)
		if err != 0 {
			fmt.Printf("r=%d r2=%d errno=%d err=%v\n", r, r2, uintptr(err), err)
		}
		return int(r), err
	}
}
