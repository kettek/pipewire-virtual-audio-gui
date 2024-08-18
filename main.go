package main

import (
	"fmt"
	"os/exec"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/pkg/errors"
)

var a fyne.App
var w fyne.Window
var devices []Device
var selectedDevice string
var virtualDevice string
var virtualDeviceId string

func main() {
	var err error
	a = app.New()
	w = a.NewWindow("Pipewire Virtual Audio")
	w.Resize(fyne.NewSize(400, 400))

	w.SetOnClosed(func() {
		deleteDevice()
	})

	devices, err = getDevices()
	if err != nil {
		panic(err)
	}

	var deviceNames []string
	for _, device := range devices {
		deviceNames = append(deviceNames, device.name)
	}

	label1 := widget.NewLabel("Virtual Device")
	value1 := widget.NewEntry()
	value1.SetText("audio-capture")
	virtualDevice = value1.Text
	value1.OnChanged = func(text string) {
		virtualDevice = text
	}
	label2 := widget.NewLabel("Capture Audio")
	value2 := widget.NewSelect(deviceNames, func(device string) {
		selectedDevice = device
	})
	button := widget.NewButton("Create & Link", func() {
		fmt.Println("Create & Link")
		device, err := currentDevice()
		if err != nil {
			dialog.ShowError(errors.Wrap(err, "capture audio not specified"), w)
			return
		}
		// Try to delete first.
		if err := deleteDevice(); err != nil {
			fmt.Println(err)
		}
		// Create it.
		if err := createDevice(); err != nil {
			dialog.ShowError(errors.Wrap(err, "createDevice"), w)
			return
		}
		// Link it.
		if err := linkDevice(device, value1.Text); err != nil {
			dialog.ShowError(errors.Wrap(err, "linkDevice"), w)
			return
		}
	})
	grid := container.New(layout.NewVBoxLayout(),
		container.New(layout.NewGridLayout(2), label1, value1),
		container.New(layout.NewGridLayout(2), label2, value2),
		layout.NewSpacer(),
		container.New(layout.NewGridLayout(1), button),
	)
	w.SetContent(grid)

	w.ShowAndRun()
}

type Device struct {
	name     string
	channels []string
}

func currentDevice() (Device, error) {
	for _, device := range devices {
		if device.name == selectedDevice {
			return device, nil
		}
	}
	return Device{}, errors.New("device not found")
}

func getDevices() ([]Device, error) {
	cmd := exec.Command("pw-link", "-o")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	outAsString := string(out)
	lines := strings.Split(outAsString, "\n")

	var devices []Device
	for _, line := range lines {
		if strings.Contains(line, "monitor") {
			continue
		}
		if strings.HasPrefix(line, "v4l2") || strings.HasPrefix(line, "alsa") || strings.HasPrefix(line, "bluez") || strings.HasPrefix(line, "Midi-Bridge") {
			continue
		}
		if !strings.Contains(line, "output") {
			continue
		}

		parts := strings.Split(line, "_")
		last := parts[len(parts)-1]
		withoutLast := strings.Join(parts[:len(parts)-1], "_")

		found := false
		for i, device := range devices {
			if device.name == withoutLast {
				devices[i].channels = append(devices[i].channels, last)
				found = true
				continue
			}
		}
		if !found {
			devices = append(devices, Device{name: withoutLast, channels: []string{last}})
		}
	}
	return devices, nil
}

func createDevice() error {
	fmt.Println("createDevice", virtualDevice)
	cmd := exec.Command("pactl", "load-module", "module-null-sink", "media.class=Audio/Source/Virtual", "sink_name="+virtualDevice, "channel_map=front-left,front-right")
	out, err := cmd.Output()
	if err != nil {
		return err
	}
	str := string(out)
	virtualDeviceId = strings.TrimSpace(str[:len(str)-1])
	return nil
}

func deleteDevice() error {
	fmt.Println("deleteDevice", virtualDeviceId)
	if virtualDeviceId == "" {
		return errors.New("virtualDeviceId not set")
	}
	cmd := exec.Command("pactl", "unload-module", virtualDeviceId)
	return cmd.Run()
}

func linkDevice(device Device, target string) error {
	var err error
	for _, channel := range device.channels {
		if channel == "FL" || channel == "FR" {
			fmt.Println("execute", "pw-link", device.name+"_"+channel, target+":input_"+channel)
			cmd := exec.Command("pw-link", device.name+"_"+channel, target+":input_"+channel)
			out, e := cmd.CombinedOutput()
			if e != nil {
				if err == nil {
					err = e
				} else {
					err = errors.Wrap(errors.Wrap(e, err.Error()), string(out))
				}
			}
		}
	}
	return err
}
