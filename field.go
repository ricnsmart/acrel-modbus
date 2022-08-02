package acrel

// Field 指安科瑞协议中没有寄存器实际地址的消息体
// 它拥有和寄存器类似的功能
type Field interface {
	// Name 字段名称
	Name() string
	// Start 字段起始位置
	Start() int
	// Len 字段长度，单位：字节
	Len() int
}

type Decoder interface {
	Decode(data []byte, values map[string]any)
}

type DecodableField interface {
	Field
	Decoder
}

type DecodableFields struct {
	len int

	fields []DecodableField
}

func NewDecodableFields(len int, fields []DecodableField) *DecodableFields {
	return &DecodableFields{
		len:    len,
		fields: fields,
	}
}

func (p *DecodableFields) Len() int {
	return p.len
}

func (p *DecodableFields) Decode(data []byte, values map[string]any) {
	for _, fd := range p.fields {
		start := fd.Start()
		end := start + fd.Len()
		fd.Decode(data[start:end], values)
	}
}
