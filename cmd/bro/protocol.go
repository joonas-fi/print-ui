package main

import (
	"encoding/binary"
)

// not for right model, but seems to document pretty well what we're dealing with:
// https://download.brother.com/welcome/docp100407/cv_ptp900_eng_raster_102.pdf

func sendInvalidate() []byte {
	hundredZeroBytes := make([]byte, 100)
	return hundredZeroBytes
}

// from docs:
// > Initializes mode settings.
func sendInitialize() []byte {
	return []byte{0x1b, 0x40}
}

func sendStatusInformationRequest() []byte {
	return []byte{0x1B, 0x69, 0x53}
}

func sendSwitchDynamicCommandMode() []byte {
	// last byte:
	// 1 = Raster mode
	// 3 = P-touch template mode
	return []byte{0x1B, 0x69, 0x61, 0x01}
}

func sendSwitchAutomaticStatusNotificationMode() []byte {
	// set automatic status notification mode to "notify"
	return []byte{0x1B, 0x69, 0x21, 0x00}
}

func sendPrintInformationCommand(imageData []byte) func() []byte {
	// TODO: assert len aligns with `chunkLen`
	dataLenInChunks := len(imageData) / chunkLen

	// print to 24mm tape [1B 69 7A {84 00 18 00 <data length 4 bytes> 00 00}]
	return func() []byte {
		// <prefix= 0x1B, 0x69, 0x7A> <n1=valid flag> <n2=media type> <n3= media width> <n4= media length>
		//
		msg := []byte{0x1B, 0x69, 0x7A, 0x84, 0x00, 0x18, 0x00}
		msg = binary.LittleEndian.AppendUint32(msg, uint32(dataLenInChunks))
		msg = append(msg, 0x00, 0x00)

		return msg
	}
}

func sendVariousModeSettings() []byte {
	// set to auto-cut, no mirror printing [1B 69 4D {40}]
	return []byte{0x1B, 0x69, 0x4D, ModeAUTOCUT}
}

func sendAdvancedModeSettings() []byte {
	// set print chaining off
	return []byte{0x1B, 0x69, 0x4B, 0x08}
}

func sendSpecifyMarginAmount() []byte {
	// set margin (feed) amount to 0 [1B 69 64 {00 00}]
	return []byte{0x1B, 0x69, 0x64, 0x00, 0x00}
}

func sendSelectCompressionMode() []byte {
	// set to TIFF compression [4D {02}]
	return []byte{0x4d, 0x02}
}

// this is where the image data is sent
func sendRasterData(imageData []byte, chunkIdx int) []byte {
	chunk := imageData[chunkLen*chunkIdx : chunkLen*chunkIdx+chunkLen]

	if byteSliceAllZeroes(chunk) { // all transparent ("unset") pixels?
		return []byte{0x5a}
	} else {
		packedChunk := packBits(chunk)
		msg := []byte{0x47}
		msg = binary.LittleEndian.AppendUint16(msg, uint16(len(packedChunk)))
		msg = append(msg, packedChunk...)
		return msg
	}
}

const chunkLen = 16

func byteSliceAllZeroes(b []byte) bool {
	for _, item := range b {
		if item != 0x00 {
			return false
		}
	}
	return true
}

func sendPrintCommandWithFeeding() []byte {
	// print and feed [1A]
	return []byte{0x1a}
}
