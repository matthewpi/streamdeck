//
// Copyright (c) 2022 Matthew Penner
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
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path"
	"regexp"
	"strconv"
	"unsafe"
)

const (
	USBHidClass = 3

	USBDevBus = "/dev/bus/usb"

	USBDevFSConnect    = 0x5517
	USBDevFSDisconnect = 0x5516
	USBDevFSClaim      = 0x8004550f
	USBDevFSRelease    = 0x80045510
	USBDevFSIoctl      = 0xc0105512
	USBDevFSBulk       = 0xc0185502
	USBDevFSControl    = 0xc0185500

	USBDescTypeDevice    = 1
	USBDescTypeConfig    = 2
	USBDescTypeString    = 3
	USBDescTypeInterface = 4
	USBDescTypeEndpoint  = 5
	USBDescTypeReport    = 33
)

type usbFSIoctl struct {
	Interface uint32
	IoctlCode uint32
	Data      uint64
}

type usbFSCtrl struct {
	ReqType uint8
	Req     uint8
	Value   uint16
	Index   uint16
	Len     uint16
	Timeout uint32
	_       uint32
	Data    uintptr
}

type usbFSBulk struct {
	Endpoint uint32
	Len      uint32
	Timeout  uint32
	Data     uintptr
}

type deviceDesc struct {
	Length            uint8
	DescriptorType    uint8
	USB               uint16
	DeviceClass       uint8
	DeviceSubClass    uint8
	DeviceProtocol    uint8
	MaxPacketSize     uint8
	Vendor            uint16
	Product           uint16
	Revision          uint16
	ManufacturerIndex uint8
	ProductIndex      uint8
	SerialIndex       uint8
	NumConfigurations uint8
}

type interfaceDesc struct {
	Length            uint8
	DescriptorType    uint8
	Number            uint8
	AltSetting        uint8
	NumEndpoints      uint8
	InterfaceClass    uint8
	InterfaceSubClass uint8
	InterfaceProtocol uint8
	InterfaceIndex    uint8
}

type endpointDesc struct {
	Length         uint8
	DescriptorType uint8
	Address        uint8
	Attributes     uint8
	MaxPacketSize  uint16
	Interval       uint8
}

func cast(b []byte, to interface{}) error {
	r := bytes.NewBuffer(b)
	return binary.Read(r, binary.LittleEndian, to)
}

func slicePtr(b []byte) uintptr {
	return uintptr(unsafe.Pointer(&b[0]))
}

var reDevBusDevice = regexp.MustCompile(`/dev/bus/usb/(\d+)/(\d+)`)

func Devices(filepath string) ([]*USB, error) {
	var devices, tmp []*USB
	files, err := os.ReadDir(filepath)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		if file.IsDir() {
			if tmp, err = Devices(path.Join(filepath, file.Name())); err != nil {
				return nil, err
			}
			devices = append(devices, tmp...)
			continue
		}
		if err := walker(path.Join(filepath, file.Name()), func(u *USB) {
			devices = append(devices, u)
		}); err != nil {
			return nil, err
		}
	}
	return devices, nil
}

func walker(path string, cb func(*USB)) error {
	desc, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	r := bytes.NewBuffer(desc)
	expected := map[byte]bool{
		USBDescTypeDevice: true,
	}
	devDesc := deviceDesc{}
	var device *USB
	for r.Len() > 0 {
		length, err := r.ReadByte()
		if err != nil {
			return err
		}
		if err := r.UnreadByte(); err != nil {
			return err
		}
		body := make([]byte, length, length)
		n, err := r.Read(body)
		if err != nil {
			return err
		}
		if n != int(length) || length < 2 {
			return errors.New("short read")
		}
		if !expected[body[1]] {
			continue
		}
		switch body[1] {
		case USBDescTypeDevice:
			expected[USBDescTypeDevice] = false
			expected[USBDescTypeConfig] = true
			if err := cast(body, &devDesc); err != nil {
				return err
			}
			//info := Info{
			//}
		case USBDescTypeConfig:
			expected[USBDescTypeInterface] = true
			expected[USBDescTypeReport] = false
			expected[USBDescTypeEndpoint] = false
			// Device left from the previous config
			if device != nil {
				cb(device)
				device = nil
			}
		case USBDescTypeInterface:
			if device != nil {
				cb(device)
				device = nil
			}
			expected[USBDescTypeEndpoint] = true
			expected[USBDescTypeReport] = true
			i := &interfaceDesc{}
			if err := cast(body, i); err != nil {
				return err
			}
			if i.InterfaceClass == USBHidClass {
				matches := reDevBusDevice.FindStringSubmatch(path)
				bus := 0
				dev := 0
				if len(matches) >= 3 {
					bus, _ = strconv.Atoi(matches[1])
					dev, _ = strconv.Atoi(matches[2])
				}
				device = &USB{
					info: DeviceInfo{
						VendorID:  devDesc.Vendor,
						ProductID: devDesc.Product,
						Revision:  devDesc.Revision,
						SubClass:  i.InterfaceSubClass,
						Protocol:  i.InterfaceProtocol,
						Interface: i.Number,
						Bus:       bus,
						Device:    dev,
					},
					path: path,
				}
			}
		case USBDescTypeEndpoint:
			if device != nil {
				if device.endpointIn != 0 && device.endpointOut != 0 {
					cb(device)
					device.endpointIn = 0
					device.endpointOut = 0
				}
				e := &endpointDesc{}
				if err := cast(body, e); err != nil {
					return err
				}
				if e.Address > 0x80 && device.endpointIn == 0 {
					device.endpointIn = e.Address
					device.inputPacketSize = e.MaxPacketSize
				} else if e.Address < 0x80 && device.endpointOut == 0 {
					device.endpointOut = e.Address
					device.outputPacketSize = e.MaxPacketSize
				}
			}
		}
	}
	if device != nil {
		cb(device)
	}
	return nil
}
