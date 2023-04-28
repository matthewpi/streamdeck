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
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
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

// Devices returns a slice of USB devices by recursively searching the given
// directory. If the directory points to a USB device, then it will be returned
// as a slice of length 1.
func Devices(dir string) ([]*USB, error) {
	s, err := os.Lstat(dir)
	if err != nil {
		return nil, err
	}

	if !s.IsDir() {
		d, err := Device(dir)
		if d != nil {
			return []*USB{d}, err
		}
		return nil, err
	}

	return devices(dir)
}

// devices .
func devices(dir string) ([]*USB, error) {
	// List contents of the directory.
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var devices []*USB
	for _, f := range files {
		path := filepath.Join(dir, f.Name())
		// If the entry is a directory, then it's a bus, so search for USB devices recursively.
		if f.IsDir() {
			devices2, err := Devices(path)
			if err != nil {
				return nil, err
			}
			devices = append(devices, devices2...)
			continue
		}

		device, err := Device(path)
		if err != nil {
			return nil, err
		}

		if device == nil {
			continue
		}

		devices = append(devices, device)
	}
	return devices, nil
}

// Device .
func Device(path string) (*USB, error) {
	f, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read device descriptor: %w", err)
	}
	r := bytes.NewBuffer(f)

	// Filter is used to filter out descriptors in order.
	filter := map[byte]bool{
		USBDescTypeDevice: true,
	}

	var (
		device *USB
		desc   deviceDesc
	)
	for r.Len() > 0 {
		length, err := r.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("failed to read byte from descriptor: %w", err)
		}

		if err := r.UnreadByte(); err != nil {
			return nil, fmt.Errorf("failed to unread descriptor length: %w", err)
		}

		b := make([]byte, length)
		n, err := r.Read(b)
		if err != nil {
			return nil, fmt.Errorf("failed to read descriptor: %w", err)
		}

		if n != int(length) || length < 2 {
			return nil, fmt.Errorf("short read from descriptor: %w", err)
		}

		// Skip descriptor that aren't in the filter.
		descriptor := b[1]
		if !filter[descriptor] {
			continue
		}

		switch descriptor {
		case USBDescTypeDevice:
			filter[USBDescTypeDevice] = false
			filter[USBDescTypeConfig] = true
			if err := cast(b, &desc); err != nil {
				return nil, err
			}
		case USBDescTypeConfig:
			filter[USBDescTypeInterface] = true
			filter[USBDescTypeReport] = false
			filter[USBDescTypeEndpoint] = false
		case USBDescTypeInterface:
			filter[USBDescTypeEndpoint] = true
			filter[USBDescTypeReport] = true

			i := &interfaceDesc{}
			if err := cast(b, i); err != nil {
				return nil, err
			}

			if i.InterfaceClass != USBHidClass {
				continue
			}

			var (
				bus int
				dev int
			)
			if matches := reDevBusDevice.FindStringSubmatch(path); len(matches) >= 3 {
				bus, _ = strconv.Atoi(matches[1])
				dev, _ = strconv.Atoi(matches[2])
			}
			device = &USB{
				info: DeviceInfo{
					VendorID:  desc.Vendor,
					ProductID: desc.Product,
					Revision:  desc.Revision,
					SubClass:  i.InterfaceSubClass,
					Protocol:  i.InterfaceProtocol,
					Interface: i.Number,
					Bus:       bus,
					Device:    dev,
				},
				path: path,
			}
		case USBDescTypeEndpoint:
			if device == nil {
				continue
			}

			if device.endpointIn != 0 && device.endpointOut != 0 {
				device.endpointIn = 0
				device.endpointOut = 0
			}

			e := &endpointDesc{}
			if err := cast(b, e); err != nil {
				return nil, err
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
	return device, nil
}
