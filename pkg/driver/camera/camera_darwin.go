package camera

import (
	"image"

	"github.com/pion/mediadevices/pkg/avfoundation"
	"github.com/pion/mediadevices/pkg/driver"
	"github.com/pion/mediadevices/pkg/frame"
	"github.com/pion/mediadevices/pkg/io/video"
	"github.com/pion/mediadevices/pkg/prop"
)

type camera struct {
	device  avfoundation.Device
	session *avfoundation.Session
	rcClose func()
}

func init() {
	Initialize()
}

// Initialize finds and registers camera devices. This is part of an experimental API.
func Initialize() {
	manager := driver.GetManager()
	for _, d := range manager.Query(driver.FilterVideoRecorder()) {
		manager.Delete(d.ID())
	}

	devices, err := avfoundation.Devices(avfoundation.Video)
	if err != nil {
		panic(err)
	}

	for _, device := range devices {
		drivers := manager.Query(func(d driver.Driver) bool {
			return d.Info().Label == device.UID
		})
		if len(drivers) > 0 {
			continue
		}

		cam := newCamera(device)
		manager.Register(cam, driver.Info{
			Label:      device.UID,
			DeviceType: driver.Camera,
			Name:       device.Name,
		})
	}
}

// StartObserver starts the background observer to monitor for device changes.
func StartObserver() error {
	manager := driver.GetManager()

	avfoundation.SetOnDeviceChange(func(device avfoundation.Device, event avfoundation.DeviceEventType) {
		switch event {
		case avfoundation.DeviceEventConnected:
			drivers := manager.Query(func(d driver.Driver) bool {
				return d.Info().Label == device.UID
			})
			if len(drivers) > 0 {
				return
			}

			cam := newCamera(device)
			manager.Register(cam, driver.Info{
				Label:      device.UID,
				DeviceType: driver.Camera,
				Name:       device.Name,
			})

		case avfoundation.DeviceEventDisconnected:
			drivers := manager.Query(func(d driver.Driver) bool {
				return d.Info().Label == device.UID
			})
			for _, d := range drivers {
				manager.Delete(d.ID())
			}
		}
	})

	return avfoundation.StartObserver()
}

func newCamera(device avfoundation.Device) *camera {
	return &camera{
		device: device,
	}
}

func (cam *camera) Open() error {
	var err error
	cam.session, err = avfoundation.NewSession(cam.device)
	return err
}

func (cam *camera) Close() error {
	if cam.rcClose != nil {
		cam.rcClose()
	}
	return cam.session.Close()
}

func (cam *camera) VideoRecord(property prop.Media) (video.Reader, error) {
	decoder, err := frame.NewDecoder(property.FrameFormat)
	if err != nil {
		return nil, err
	}

	rc, err := cam.session.Open(property)
	if err != nil {
		return nil, err
	}
	cam.rcClose = rc.Close
	r := video.ReaderFunc(func() (image.Image, func(), error) {
		frame, _, err := rc.Read()
		if err != nil {
			return nil, func() {}, err
		}
		return decoder.Decode(frame, property.Width, property.Height)
	})
	return r, nil
}

func (cam *camera) Properties() []prop.Media {
	return cam.session.Properties()
}
