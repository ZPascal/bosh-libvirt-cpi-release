package vm

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
	"time"
)

// More or less vendored from https://github.com/johto/iso9660wrap/blob/master/iso9660wrap.go
type ISO9660 struct {
	FileName string
	Contents []byte
}

const (
	volumeDescriptorSetMagic              = "\x43\x44\x30\x30\x31\x01"
	primaryVolumeSectorNum         uint32 = 16
	numVolumeSectors               uint32 = 2 // primary + terminator
	littleEndianPathTableSectorNum uint32 = primaryVolumeSectorNum + numVolumeSectors
	bigEndianPathTableSectorNum    uint32 = littleEndianPathTableSectorNum + 1
	numPathTableSectors                   = 2 // no secondaries
	rootDirectorySectorNum         uint32 = primaryVolumeSectorNum + numVolumeSectors + numPathTableSectors
)

func (i ISO9660) inputLen() uint32 {
	return uint32(len(i.Contents))
}

func (i ISO9660) Bytes() ([]byte, error) {
	i.FileName = strings.ToUpper(i.FileName)

	if !i.fileNameSatisfiesISOConstraints(i.FileName) {
		return nil, fmt.Errorf("file name '%s' violates ISO9660 constraints", i.FileName)
	}

	buf := bytes.NewBuffer([]byte{})
	bufw := bufio.NewWriter(buf)
	w := NewISO9660Writer(bufw)

	err := i.writePrimaryVolumeDescriptor(w)
	if err != nil {
		return nil, err
	}

	err = i.writeVolumeDescriptorSetTerminator(w)
	if err != nil {
		return nil, err
	}

	if err := i.writePathTable(w, binary.LittleEndian); err != nil {
		return nil, err
	}
	if err := i.writePathTable(w, binary.BigEndian); err != nil {
		return nil, err
	}

	err = i.writeData(w)
	if err != nil {
		return nil, err
	}

	w.Finish()
	if err := bufw.Flush(); err != nil {
		return nil, err
	}

	reservedBytes := make([]byte, int64(16*SectorSize))

	return append(reservedBytes, buf.Bytes()...), nil
}

func (i ISO9660) writePrimaryVolumeDescriptor(w *ISO9660Writer) error {
	if len(i.FileName) > 32 {
		i.FileName = i.FileName[:32]
	}
	now := time.Now()

	sw, err := w.NextSector()
	if err != nil {
		return err
	}
	if w.CurrentSector() != primaryVolumeSectorNum {
		return fmt.Errorf("internal error: unexpected primary volume sector %d", w.CurrentSector())
	}

	if err := sw.WriteByte('\x01'); err != nil {
		return err
	}
	if _, err := sw.WriteString(volumeDescriptorSetMagic); err != nil {
		return err
	}
	if err := sw.WriteByte('\x00'); err != nil {
		return err
	}

	if _, err := sw.WritePaddedString("", 32); err != nil {
		return err
	}
	if _, err := sw.WritePaddedString(i.FileName, 32); err != nil {
		return err
	}

	if _, err := sw.WriteZeros(8); err != nil {
		return err
	}
	if _, err := sw.WriteBothEndianDWord(i.numTotalSectors()); err != nil {
		return err
	}
	if _, err := sw.WriteZeros(32); err != nil {
		return err
	}

	if _, err := sw.WriteBothEndianWord(1); err != nil { // volume set size
		return err
	}
	if _, err := sw.WriteBothEndianWord(1); err != nil { // volume sequence number
		return err
	}
	if _, err := sw.WriteBothEndianWord(uint16(SectorSize)); err != nil {
		return err
	}
	if _, err := sw.WriteBothEndianDWord(SectorSize); err != nil { // path table length
		return err
	}

	if _, err := sw.WriteLittleEndianDWord(littleEndianPathTableSectorNum); err != nil {
		return err
	}
	if _, err := sw.WriteLittleEndianDWord(0); err != nil { // no secondary path tables
		return err
	}
	if _, err := sw.WriteBigEndianDWord(bigEndianPathTableSectorNum); err != nil {
		return err
	}
	if _, err := sw.WriteBigEndianDWord(0); err != nil { // no secondary path tables
		return err
	}

	if _, err := WriteDirectoryRecord(sw, "\x00", rootDirectorySectorNum); err != nil { // root directory
		return err
	}

	for _, s := range []string{"", "", "", ""} { // volume set, publisher, data preparer, application identifiers
		if _, err := sw.WritePaddedString(s, 128); err != nil {
			return err
		}
	}

	for _, s := range []string{"", "", ""} { // copyright, abstract, bibliographical file identifiers
		if _, err := sw.WritePaddedString(s, 37); err != nil {
			return err
		}
	}

	if _, err := sw.WriteDateTime(now); err != nil { // volume creation
		return err
	}
	if _, err := sw.WriteDateTime(now); err != nil { // most recent modification
		return err
	}
	if _, err := sw.WriteUnspecifiedDateTime(); err != nil { // expires
		return err
	}
	if _, err := sw.WriteUnspecifiedDateTime(); err != nil { // is effective (?)
		return err
	}

	if err := sw.WriteByte('\x01'); err != nil { // version
		return err
	}
	if err := sw.WriteByte('\x00'); err != nil { // reserved
		return err
	}

	if _, err := sw.PadWithZeros(); err != nil { // 512 (reserved for app) + 653 (zeros)
		return err
	}

	return nil
}

func (i ISO9660) writeVolumeDescriptorSetTerminator(w *ISO9660Writer) error {
	sw, err := w.NextSector()
	if err != nil {
		return err
	}
	if w.CurrentSector() != primaryVolumeSectorNum+1 {
		return fmt.Errorf("internal error: unexpected volume descriptor set terminator sector %d", w.CurrentSector())
	}

	if err := sw.WriteByte('\xFF'); err != nil {
		return err
	}
	if _, err := sw.WriteString(volumeDescriptorSetMagic); err != nil {
		return err
	}

	if _, err := sw.PadWithZeros(); err != nil {
		return err
	}

	return nil
}

func (i ISO9660) writePathTable(w *ISO9660Writer, bo binary.ByteOrder) error {
	sw, err := w.NextSector()
	if err != nil {
		return err
	}
	if err := sw.WriteByte(1); err != nil { // name length
		return err
	}
	if err := sw.WriteByte(0); err != nil { // number of sectors in extended attribute record
		return err
	}
	if _, err := sw.WriteDWord(bo, rootDirectorySectorNum); err != nil {
		return err
	}
	if _, err := sw.WriteWord(bo, 1); err != nil { // parent directory recno (root directory)
		return err
	}
	if err := sw.WriteByte(0); err != nil { // identifier (root directory)
		return err
	}
	if err := sw.WriteByte(1); err != nil { // padding
		return err
	}
	if _, err := sw.PadWithZeros(); err != nil {
		return err
	}
	return nil
}

func (i ISO9660) writeData(w *ISO9660Writer) error {
	sw, err := w.NextSector()
	if err != nil {
		return err
	}
	if w.CurrentSector() != rootDirectorySectorNum {
		return fmt.Errorf("internal error: unexpected root directory sector %d", w.CurrentSector())
	}

	if _, err := WriteDirectoryRecord(sw, "\x00", w.CurrentSector()); err != nil {
		return err
	}
	if _, err := WriteDirectoryRecord(sw, "\x01", rootDirectorySectorNum); err != nil {
		return err
	}
	if _, err := WriteFileRecordHeader(sw, i.FileName, w.CurrentSector()+1, i.inputLen()); err != nil {
		return err
	}

	inputBuf := bytes.NewBuffer(i.Contents)

	// Now stream the data.  Note that the first buffer is never of SectorSize,
	// since we've already filled a part of the sector.
	b := make([]byte, SectorSize)
	total := uint32(0)
	for {
		l, err := inputBuf.Read(b)
		if err != nil && err != io.EOF {
			return fmt.Errorf("could not read from input file: %s", err)
		}
		if l > 0 {
			sw, err := w.NextSector()
			if err != nil {
				return err
			}
			if _, err := sw.Write(b[:l]); err != nil {
				return err
			}
			total += uint32(l)
		}
		if err == io.EOF {
			break
		}
	}
	if total != i.inputLen() {
		return fmt.Errorf("input file size changed (expected to read %d, read %d)", i.inputLen(), total)
	} else if w.CurrentSector() != i.numTotalSectors()-1 {
		return fmt.Errorf("internal error: unexpected last sector number (expected %d, actual %d)", i.numTotalSectors()-1, w.CurrentSector())
	}

	return nil
}

func (i ISO9660) numTotalSectors() uint32 {
	numDataSectors := (i.inputLen() + (SectorSize - 1)) / SectorSize
	return 1 + rootDirectorySectorNum + numDataSectors
}

func (ISO9660) fileNameSatisfiesISOConstraints(filename string) bool {
	invalidCharacter := func(r rune) bool {
		// According to ISO9660, only capital letters, digits, and underscores
		// are permitted.  Some sources say a dot is allowed as well.  I'm too
		// lazy to figure it out right now.
		if r >= 'A' && r <= 'Z' {
			return false
		} else if r >= '0' && r <= '9' {
			return false
		} else if r == '_' {
			return false
		} else if r == '.' {
			return false
		}
		return true
	}
	return strings.IndexFunc(filename, invalidCharacter) == -1
}
