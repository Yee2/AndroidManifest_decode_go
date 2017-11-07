package apkxmldecode

import (
	"fmt"
	"io"
	"io/ioutil"
	"encoding/binary"
	"bytes"
	"math"
	"strconv"
	"errors"
	"strings"
)

type Manifest struct {
	Strings   []string
	Namespace []StartNamespaceChunk
	StartTagChunk []StartTagChunkAttributes
	XML *bytes.Buffer
}

type StartNamespaceChunk struct {
	ChunkType  [4]byte "Chunk的类型，固定四个字节：0x00100100"
	ChunkSize  uint32 "Chunk的大小，四个字节"
	LineNumber uint32 "在AndroidManifest文件中的行号，四个字节"
	Unknown    [4]byte "未知区域，四个字节"
	Prefix     uint32 "命名空间的前缀(在字符串中的索引值)，比如：android"
	Uri        uint32 "命名空间的uri(在字符串中的索引值)：比如：http://schemas.android.com/apk/res/android"
}
type StringChunk struct {
	ChunkType        [4]byte  "StringChunk的类型，固定四个字节：0x001C0001"
	ChunkSize        uint32  "StringChunk的大小，四个字节"
	StringCount      uint32 "StringChunk中字符串的个数，四个字节"
	StyleCount       uint32 "StringChunk中样式的个数，四个字节"
	Unknown          [4]byte "位置区域，四个字节，在解析的过程中，这里需要略过四个字节"
	StringPoolOffset uint32 "字符串池的偏移值，四个字节"
	StylePoolOffset  uint32 "样式池的偏移值"
}
type StartTagChunk struct {
	ChunkType      uint32 "Chunk的类型，固定四个字节：0x00100102"
	ChunkSize      uint32 "Chunk的大小，固定四个字节"
	LineNumber     uint32 "对应于AndroidManifest中的行号，四个字节"
	Unknown        [4]byte "未知领域，四个字节"
	NamespaceUri   uint32 "这个标签用到的命名空间的Uri,比如用到了android这个前缀，那么就需要用http://schemas.android.com/apk/res/android这个Uri去获取，四个字节"
	Name           uint32 "标签名称(在字符串中的索引值)，四个字节"
	Flags          uint32 "标签的类型，四个字节，比如是开始标签还是结束标签等"
	AttributeCount uint32 "标签包含的属性个数，四个字节"
	ClassAttribute uint32 "标签包含的类属性，四个字节"
}
type EndTagChunk struct {
	ChunkType      uint32 "Chunk的类型，固定四个字节"
	ChunkSize      uint32 "Chunk的大小，固定四个字节"
	LineNumber     uint32 "对应于AndroidManifest中的行号，四个字节"
	Unknown        [4]byte "未知领域，四个字节"
	NamespaceUri   uint32 "这个标签用到的命名空间的Uri,比如用到了android这个前缀，那么就需要用http://schemas.android.com/apk/res/android这个Uri去获取，四个字节"
	Name           uint32 "标签名称(在字符串中的索引值)，四个字节"
}

type Attribute struct {
	Uri   uint32
	Name  uint32
	Value uint32
	Type  uint32
	Data  uint32
}

type StartTagChunkAttributes struct {
	StartTagChunk
	Attributes []Attribute
}
func (attr Attribute)value(m *Manifest) string {
	switch attr.Type {
	case AttributeType.BOOLEAN:
		if attr.Value != 0{
			return "true"
		}else{
			return "false"
		}
	case AttributeType.STRING:
		return m.get(attr.Value)
	case AttributeType.FLOAT:
		return fmt.Sprintf("%f",math.Float32frombits(attr.Data))
	case AttributeType.INT:
		return strconv.FormatInt(int64(attr.Data),10)
	case AttributeType.RESOURCE:
		//fmt.Printf("RESOURCE %d :%s %+v\n",attr.Type >> 24 ,m.get(attr.Name),attr)
		if prefix := m.prefix(attr.Uri); attr.Data >>24 == 1 && prefix != ""{
			return fmt.Sprintf("@%s:%08X",prefix,attr.Data)
		}
		return fmt.Sprintf("@%08X",attr.Data)
	case AttributeType.FLAGS:
		return fmt.Sprintf("0x%08X",attr.Data)
	default:
		return m.get(attr.Value)
	}
}
var AttributeType = struct {
	STRING,
	INT,
	RESOURCE,
	BOOLEAN,
	ATTR,
	DIMEN,
	FRACTION,
	FLOAT,
	FLAGS,
	COLOR1,
	COLOR2 uint32
}{
	0x03000008,
	0x10000008,
	0x01000008,
	0x12000008,
	0x02000008,
	0x05000008,
	0x06000008,
	0x04000008,
	0x11000008,
	0x1C000008,
	0x1D000008,
}
var indent int

func New(r io.Reader)(*Manifest, error) {
	var offset uint32
	data, err := ioutil.ReadAll(r)
	if err != nil{
		return nil,err
	}
	//reader := bytes.NewReader(data)
	MagicNumber := data[offset:offset+4]
	offset += 4
	if binary.LittleEndian.Uint32(MagicNumber) != 0x00080003  {
		return nil,errors.New("wrong format!")
	}
	FileSize := binary.LittleEndian.Uint32(data[4:8])
	offset += 4
	fmt.Printf("file size:%d byte\n", FileSize)

	var res  = &Manifest{XML:new(bytes.Buffer)}
	res.XML.Write([]byte("<?xml version=\"1.0\" encoding=\"utf-8\"?>\n"))
	indent = -4
	for i := offset; int(i) < len(data); {
		_, ChunkSize := data[i:i+4], binary.LittleEndian.Uint32(data[i+4:i+8])
		//fmt.Printf("chunk type:%+v chunk size:%d bytes\n",ChunkType,ChunkSize)
		var Chunk []byte
		if int(i+ChunkSize) < len(data) {
			Chunk = data[i:i+ChunkSize]
		} else {
			Chunk = data[i:]
		}

		switch binary.LittleEndian.Uint32(Chunk[0:4]) {
		case 0x001C0001:
			res.Strings = chunkString(Chunk)
			//fmt.Printf("%+v\n%s\n",res)
		case 0x00100100:
			res.Namespace = append(res.Namespace, chunkStartNamespace(Chunk))
		case 0x00100102:
			a, b := chunkStratTag(Chunk)
			res.StartTagChunk = append(res.StartTagChunk, StartTagChunkAttributes{StartTagChunk: a, Attributes: b})
			if err = add2xml(res.XML,res,StartTagChunkAttributes{StartTagChunk: a, Attributes: b}); err != nil {
				return nil,err;
			}
		case 0x00100103:
			endtag := chunkEndTag(Chunk)
			add2xml(res.XML,res,endtag)
		}
		i += ChunkSize
	}
	return res,nil
}
func (m Manifest) String()string{
	return m.XML.String()
}
func (m Manifest) Read(p []byte)(int,error){
	return m.XML.Read(p)
}
func (m Manifest) get(index uint32) (string) {
	if len(m.Strings) > int(index) {
		return m.Strings[index]
	}
	return ""
}
func (m Manifest) prefix(index uint32) (string) {
	for _, v := range m.Namespace {
		if v.Uri == index {
			if len(m.Strings) > int(v.Prefix) {
				return m.Strings[v.Prefix]

			}
			return ""
		}
	}
	return ""
}
func add2xml(writer io.Writer, m *Manifest,tag interface{}) error {
	switch t := tag.(type) {
	case StartTagChunkAttributes:
		if name := m.get(t.Name); name != ""{
			indent += 4
			writer.Write([]byte(fmt.Sprintf("%s<%s",strings.Repeat(" ",indent),name)))
			indent += 4
			nobr := true
			if name == "manifest"{
				for _, v := range m.Namespace {
					if nobr{
						writer.Write([]byte(" "))
						nobr = false
					}else{
						writer.Write([]byte("\n" + strings.Repeat(" ",indent)))
					}
					// FIXME: 使用更优雅的实现方式
					if len(m.Strings) > int(v.Prefix) && len(m.Strings) > int(v.Uri) {
						writer.Write([]byte(fmt.Sprintf("xmlns:%s=\"%s\"",m.Strings[v.Prefix],m.Strings[v.Uri])))
					}
				}
			}
			for _,attr := range t.Attributes{
				attrName := m.get(attr.Name)
				if attrName == ""{
					return errors.New("Failed to get label name.")
				}
				if nobr{
					writer.Write([]byte(" "))
					nobr = false
				}else{
					writer.Write([]byte("\n" + strings.Repeat(" ",indent)))
				}
				if prefix := m.prefix(attr.Uri); prefix != ""{
					writer.Write([]byte(fmt.Sprintf("%s:%s=\"%s\"",prefix,attrName,attr.value(m))))
				}else{
					writer.Write([]byte(fmt.Sprintf("%s=\"%s\"",attrName,attr.value(m))))
				}
			}
			indent -= 4
			writer.Write([]byte(fmt.Sprintf(">\n")))
		}else{
			return errors.New("Failed to get label name.")
		}
	case EndTagChunk:
		if name := m.get(t.Name); name != ""{
			writer.Write([]byte(fmt.Sprintf("%s</%s>\n",strings.Repeat(" ",indent),name)))
		}else{
			return errors.New("Failed to get label name.")
		}
		indent -= 4
	default:
		return errors.New("Wrong data")
	}
	return nil
}
func chunkEndTag(b []byte) EndTagChunk {
	var ETC EndTagChunk
	r := bytes.NewReader(b)
	err := binary.Read(r, binary.LittleEndian, &ETC)
	if err != nil {
		panic(err)
	}
	return ETC
}
func chunkStratTag(b []byte) (StartTagChunk, []Attribute) {
	var STC StartTagChunk
	r := bytes.NewReader(b)
	err := binary.Read(r, binary.LittleEndian, &STC)
	if err != nil {
		panic(err)
	}
	attrs := make([]Attribute, 0, STC.AttributeCount)
	for i := uint32(0); i < STC.AttributeCount; i++ {
		data := b[36+i*20:36+i*20+20]
		attr := Attribute{}
		binary.Read(bytes.NewReader(data), binary.LittleEndian, &attr)
		//attr.Type >>= 24
		attrs = append(attrs, attr)
	}
	return STC, attrs
}
func chunkStartNamespace(b []byte) StartNamespaceChunk {
	var SNC StartNamespaceChunk
	r := bytes.NewReader(b)
	err := binary.Read(r, binary.LittleEndian, &SNC)
	if err != nil {
		panic(err)
	}
	return SNC
}
func chunkString(b []byte) []string {
	SC := StringChunk{}
	r := bytes.NewReader(b)
	binary.Read(r, binary.LittleEndian, &SC)
	res := make([]string, 0, SC.StringCount)
	for i := SC.StringPoolOffset; int(i) < len(b); {
		size := uint32(binary.LittleEndian.Uint16(b[i:i+2])) * 2
		var chunk []byte
		if int(i+size) < len(b) {
			chunk = b[i+2:i+2+size]
		} else {
			chunk = b[i+2:]
		}
		str := bytes.Buffer{}
		for _,i := range chunk{
			if i != 0x00{
				str.WriteByte(i)
			}
		}
		res = append(res, str.String())
		i += size + 4
	}
	return res
}
