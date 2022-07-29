package acrel

import (
	"encoding/binary"
	"fmt"
)

/*
安科瑞 Modbus对接协议
*/

/*基本格式
{{ 命令字（1字节）消息体（可变）校验位（2字节） }}
7b 7b x xxx xx  7d 7d

校验位长度为 2 个字节，Modbus CRC 校验算法 。校验范围为命令字开始（含命令字）到消息体结束。校验位采用小端模式。

注：任何与服务器进行交互的数据都需要按此格式进行编解码。（包括透传，注册等等）
*/

type Frame struct {
	Function uint8
	Data     []byte
}

// NewFrame converts a packet to a Acrel frame.
func NewFrame(packet []byte) (*Frame, error) {
	// Check the that the packet length.
	if len(packet) < 7 {
		return nil, fmt.Errorf("acrel: frame error: packet less than 7 bytes: 0x% x", packet)
	}

	pLen := len(packet)

	// 检查 Delimiters 定界符
	if packet[0] != '{' || packet[1] != '{' ||
		packet[pLen-2] != '}' || packet[pLen-1] != '}' {
		return nil, fmt.Errorf("acrel: 定界符错误：0x% x", packet)
	}

	// Check the CRC.
	crcExpect := binary.LittleEndian.Uint16(packet[pLen-4 : pLen-2])
	crcCalc := crcModbus(packet[2 : pLen-4])

	if crcCalc != crcExpect {
		return nil, fmt.Errorf("acrel: frame error: CRC (expected 0x%x, got 0x%x)", crcExpect, crcCalc)
	}

	frame := &Frame{
		Function: packet[2],
		Data:     packet[3 : pLen-4],
	}

	return frame, nil
}

func (frame *Frame) Copy() *Frame {
	f := *frame
	return &f
}

// Bytes returns the MODBUS byte stream based on the AcrelFrame fields
func (frame *Frame) Bytes() []byte {
	b := make([]byte, 3)

	// 添加定界符
	b[0] = '{'
	b[1] = '{'
	b[2] = frame.Function

	b = append(b, frame.Data...)

	// Calculate the CRC.
	pLen := len(b)
	crc := crcModbus(b[2:pLen])

	// Add the CRC.
	d := make([]byte, 2)
	binary.LittleEndian.PutUint16(d, crc)

	b = append(b, d...)
	b = append(b, '}', '}')
	return b
}

// GetFunction returns the Modbus function code.
func (frame *Frame) GetFunction() uint8 {
	return frame.Function
}

// GetData returns the AcrelFrame Data byte field.
func (frame *Frame) GetData() []byte {
	return frame.Data
}

// SetData sets the AcrelFrame Data byte field and updates the frame length
// accordingly.
func (frame *Frame) SetData(data []byte) {
	frame.Data = data
}
