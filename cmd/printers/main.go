package main

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"log"
	"os"

	"github.com/function61/gokit/app/cli"
	"github.com/function61/gokit/app/dynversion"
	"github.com/function61/gokit/os/osutil"
	"github.com/spf13/cobra"
	"go.bug.st/serial"
)

func fillRectBlack(im *image.RGBA, r image.Rectangle) {
	draw.Draw(im, r, &image.Uniform{color.Black}, image.Point{0, 0}, draw.Src)
}

const (
	drawingHeight = 128
	// looks like 128 px height has 64 px of height available
	deadZoneTopAndBottom = 32
	leftMargin           = 30
)

func main() {
	app := &cobra.Command{
		Use: os.Args[0],
		// Short:   "Description for app",
		Version: dynversion.Version,
	}

	app.AddCommand(debugEntrypoint1())
	app.AddCommand(debugEntrypoint2())

	app.AddCommand(&cobra.Command{
		Use: "print [image]",
		// Short: "Reticulates splines",
		Args: cobra.ExactArgs(1),
		Run: cli.Runner(func(_ context.Context, args []string, _ *log.Logger) error {
			return printFile(args[0])
		}),
	})

	app.AddCommand(warehouseEntrypoint())
	app.AddCommand(warehouseEntrypoint2())
	app.AddCommand(netumUIEntrypoint())

	app.AddCommand(genIdsEntrypoint())

	osutil.ExitIfError(app.Execute())
}

func printFile(imagePath string) error {
	// imageData, err := getImageData(imagePath, isBlack)
	imageData, err := getImageData(imagePath, isBlack)
	if err != nil {
		return err
	}

	// doesn't work with just file open
	// printerConn, err := os.OpenFile("/dev/rfcomm0", os.O_APPEND, 0)
	printerConn, err := serial.Open("/dev/rfcomm0", &serial.Mode{})
	if err != nil {
		return fmt.Errorf("serial open: %w\n\n  pro-tip: probably need $ sudo rfcomm bind 0 <bluetooth address>", err)
	}
	defer printerConn.Close()

	if err := configurePrinter(imageData, func(msg []byte) error {
		_, err := printerConn.Write(msg)
		return err
	}); err != nil {
		return err
	}

	for chunkIdx := 0; chunkIdx < len(imageData)/chunkLen; chunkIdx++ {
		if _, err := printerConn.Write(sendRasterData(imageData, chunkIdx)); err != nil {
			return err
		}
	}

	if _, err := printerConn.Write(sendPrintCommandWithFeeding()); err != nil {
		return err
	}

	return nil
}

func configurePrinter(imageData []byte, send func(msg []byte) error) error {
	for _, fn := range []func() []byte{
		sendInvalidate,
		sendInitialize,
		sendStatusInformationRequest,
		sendSwitchDynamicCommandMode,
		sendSwitchAutomaticStatusNotificationMode,
		sendPrintInformationCommand(imageData),
		sendVariousModeSettings,
		sendAdvancedModeSettings,
		sendSpecifyMarginAmount,
		sendSelectCompressionMode,
	} {
		if err := send(fn()); err != nil {
			return err
		}
	}

	return nil
}
