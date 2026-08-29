package main

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/function61/gokit/testing/assert"
)

func TestAgainstReferenceImplementation(t *testing.T) {
	imageData, err := getImageData("testdata/Flatpak_Logo.svg.png", alphaToBlack)
	assert.Ok(t, err)
	// imageData=make([]byte,128*128/8)

	// same order as `referenceImplementationDatagrams`
	orderOfFns := []func() []byte{
		sendInvalidate,
		sendInitialize,
		sendStatusInformationRequest,
		sendSwitchDynamicCommandMode,
		sendSwitchAutomaticStatusNotificationMode,
		sendPrintInformationCommand(imageData),
		sendVariousModeSettings,
		sendAdvancedModeSettings,
		sendSpecifyMarginAmount,
		sendSelectCompressionMode, // datagramIdx==9

		// we don't want to repeat 128 times sendRasterData() here, so it'll be explicitly handled by the code below

		// sendRasterData, // for phases 10 onwards for the next 128 datagrams
		// sendPrintCommandWithFeeding, // last one
	}

	makeDatagram := func(imageData []byte, datagramIdx int) []byte {
		switch {
		case datagramIdx < len(orderOfFns):
			return orderOfFns[datagramIdx]()
		case datagramIdx < len(orderOfFns)+128:
			return sendRasterData(imageData, datagramIdx-len(orderOfFns))
		default: // last one
			return sendPrintCommandWithFeeding()
		}
	}

	for i := 0; i < len(referenceImplementationDatagrams); i++ {
		sentByUs := makeDatagram(imageData, i)
		sentByReferenceImplementation, err := hex.DecodeString(referenceImplementationDatagrams[i])
		assert.Ok(t, err)

		if !bytes.Equal(sentByUs, sentByReferenceImplementation) {
			t.Errorf("datagram[%d]: %x (ref) %x (us)", i, sentByReferenceImplementation, sentByUs)
		}
	}
}

// NOTE: string concatenations mean multiple datagram-writes collapsed into one
var referenceImplementationDatagrams = []string{
	"00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000", // sendInvalidate
	"1b40",                                 // sendInitialize
	"1b6953",                               // sendStatusInformationRequest
	"1b696101",                             // sendSwitchDynamicCommandMode
	"1b692100",                             // sendSwitchAutomaticStatusNotificationMode
	"1b697a84001800" + "80000000" + "0000", // sendPrintInformationCommand
	"1b694d" + "40",                        // sendVariousModeSettings
	"1b694b08",                             // sendAdvancedModeSettings
	"1b69640000",                           // sendSpecifyMarginAmount
	"4d02",                                 // sendSelectCompressionMode
	"5a",                                   // this and next 128 are sendRasterData
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"470800fd00fdff0080fa00",
	"470a00fe000003fdff00e0fa00",
	"470a00fe000007fdff00f0fa00",
	"470a00fe00000ffdff00f8fa00",
	"470f00fe00000ffdff00f8fe00011ffeff00",
	"470f00fe00001ffdff00fcfe00011ffeff00",
	"470f00fe00001ffdff00fefe00011ffeff00",
	"471100fe00013ff0ff000107fefe00011dc0ff00",
	"471100fe00017ff0ff000101fffe00011dc0ff00",
	"471100fe00017ff8ff000101fffe00011dc0ff00",
	"471100fe0001fff8fe0001ff80ff00011dc0ff00",
	"471100fe0001fffcfe0001ffc0ff00011dc0ff00",
	"470d00ff000201fffefe00017fc0fb00",
	"471200ff000203fffefe00017fe0ff00011ffeff00",
	"471200ff000003fffffe00013fe0ff00011ffeff00",
	"471200ff000007fffffe00011ff0ff00011ffeff00",
	"470f00ff000007ffff0080ff00011ff0fb00",
	"471400ff00000fffff0080ff00010ff8ff000101bcff00",
	"471400ff00000fffff00c0ff00010ffcff000103feff00",
	"471400ff00001fffff00e0ff000107fcff000107feff00",
	"471400ff00003fffff00e0ff000103feff0001076eff00",
	"471400ff00003fffff00f0ff000103feff0001076eff00",
	"471400ff00007fffff00f0ff000101ffff000107feff00",
	"471400ff00007fffff00f8ff000101ffff000103feff00",
	"471000ff00feff00fcfe0004ff800001feff00",
	"470f00ff00feff00fcfe0003ffc00007fe00",
	"471100010001feff00fefe00047fc0001ff8ff00",
	"471100010001feff00fefe00043fe0001ffeff00",
	"470f00010003fdfffe00043fe0001ffeff00",
	"470f00010003fdfffe00041fe0001ffeff00",
	"471100010003fdff0080ff00041fe000070eff00",
	"470c00020003fcfe00fdff00e0fc00",
	"471100020003fcff000001fdff05e00007ffc000",
	"471100020003feff000003fdff05e00007ffc000",
	"471100020001ffff000003fdff05e00007ffc000",
	"471100020001ffff000007fdff03c000039eff00",
	"471000ff0003ff800007fdff03c000070eff00",
	"471000ff0003ff80000ffdff038000079eff00",
	"471000ff00037fc0000ffdffff000103feff00",
	"471000ff00037fc0001ffdffff000103fcff00",
	"471200ff00033fe0003ffeff00feff000101f8ff00",
	"471100ff00033ff0003ffeff00fefe00003cff00",
	"471200ff00031ff0007ffeff00fcff000103feff00",
	"471200ff00030ff8007ffeff00fcff000103feff00",
	"471100ff00020ff800fdff00f8ff000107feff00",
	"471100ff000207fc01fdff00f0ff0001076eff00",
	"471100ff000207fc01fdff00f0ff000107feff00",
	"471100ff000203fe03fdff00e0ff000103feff00",
	"471100ff000203ff03fdff00e0ff000103feff00",
	"471100ff000201ff07fdff00c0ff000101feff00",
	"470b00fe0001ff87fdff00c0fb00",
	"471000fe0001ff8ffdff0080ff00011ffeff00",
	"470e00fe00017fdffdfffe00011ffeff00",
	"470d00fe00007ffcfffe00011ffeff00",
	"470f00fe00003ffdff00fefe000101f8ff00",
	"470f00fe00003ffdff00fefe000103fcff00",
	"470f00fe00001ffdff00fcfe000107feff00",
	"470f00fe00000ffdff00f8fe0001079eff00",
	"470f00fe00000ffdff00f8fe0001060eff00",
	"470e00fe000007fdff00f0fd000002ff00",
	"470a00fe000003fdff00e0fa00",
	"470800fd00fdff0080fa00",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"5a",
	"1a"}
