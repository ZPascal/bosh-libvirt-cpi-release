package vm

// more or less vendored from github.com/johto/iso9660wrap/blob/master/directories.go

import (
	"fmt"
	"time"
)

func WriteDirectoryRecord(w *SectorWriter, identifier string, firstSectorNum uint32) (uint32, error) {
	if len(identifier) > 30 {
		return 0, fmt.Errorf("directory identifier length %d is out of bounds", len(identifier))
	}
	recordLength := 33 + len(identifier)

	if err := w.WriteByte(byte(recordLength)); err != nil {
		return 0, err
	}
	if err := w.WriteByte(0); err != nil { // number of sectors in extended attribute record
		return 0, err
	}
	if _, err := w.WriteBothEndianDWord(firstSectorNum); err != nil {
		return 0, err
	}
	if _, err := w.WriteBothEndianDWord(SectorSize); err != nil { // directory length
		return 0, err
	}
	if err := writeDirectoryRecordtimestamp(w, time.Now()); err != nil {
		return 0, err
	}
	if err := w.WriteByte(byte(3)); err != nil { // bitfield; directory
		return 0, err
	}
	if err := w.WriteByte(byte(0)); err != nil { // file unit size for an interleaved file
		return 0, err
	}
	if err := w.WriteByte(byte(0)); err != nil { // interleave gap size for an interleaved file
		return 0, err
	}
	if _, err := w.WriteBothEndianWord(1); err != nil { // volume sequence number
		return 0, err
	}
	if err := w.WriteByte(byte(len(identifier))); err != nil {
		return 0, err
	}
	if _, err := w.WriteString(identifier); err != nil {
		return 0, err
	}
	// optional padding to even length
	if recordLength%2 == 1 {
		recordLength++
		if err := w.WriteByte(0); err != nil {
			return 0, err
		}
	}
	return uint32(recordLength), nil
}

func WriteFileRecordHeader(w *SectorWriter, identifier string, firstSectorNum uint32, fileSize uint32) (uint32, error) {
	if len(identifier) > 30 {
		return 0, fmt.Errorf("directory identifier length %d is out of bounds", len(identifier))
	}
	recordLength := 33 + len(identifier)

	if err := w.WriteByte(byte(recordLength)); err != nil {
		return 0, err
	}
	if err := w.WriteByte(0); err != nil { // number of sectors in extended attribute record
		return 0, err
	}
	if _, err := w.WriteBothEndianDWord(firstSectorNum); err != nil { // first sector
		return 0, err
	}
	if _, err := w.WriteBothEndianDWord(fileSize); err != nil {
		return 0, err
	}
	if err := writeDirectoryRecordtimestamp(w, time.Now()); err != nil {
		return 0, err
	}
	if err := w.WriteByte(byte(0)); err != nil { // bitfield; normal file
		return 0, err
	}
	if err := w.WriteByte(byte(0)); err != nil { // file unit size for an interleaved file
		return 0, err
	}
	if err := w.WriteByte(byte(0)); err != nil { // interleave gap size for an interleaved file
		return 0, err
	}
	if _, err := w.WriteBothEndianWord(1); err != nil { // volume sequence number
		return 0, err
	}
	if err := w.WriteByte(byte(len(identifier))); err != nil {
		return 0, err
	}
	if _, err := w.WriteString(identifier); err != nil {
		return 0, err
	}
	// optional padding to even length
	if recordLength%2 == 1 {
		recordLength++
		if err := w.WriteByte(0); err != nil {
			return 0, err
		}
	}
	return uint32(recordLength), nil
}

func writeDirectoryRecordtimestamp(w *SectorWriter, t time.Time) error {
	t = t.UTC()
	for _, b := range []byte{
		byte(t.Year() - 1900),
		byte(t.Month()),
		byte(t.Day()),
		byte(t.Hour()),
		byte(t.Minute()),
		byte(t.Second()),
		0, // UTC offset
	} {
		if err := w.WriteByte(b); err != nil {
			return err
		}
	}
	return nil
}
