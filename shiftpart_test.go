package main

import (
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/hlubek/readercomp"
)

func uniqBuffer(buffer []byte, ofs int, end int, uniq string) {
	inBuf := 0
	for i := 0; inBuf < int(end); i += 1 {
		out := fmt.Sprintf("%c%d", uniq[i%len(uniq)], i)
		if inBuf+len(out) >= int(end) {
			out = out[:int(end)-inBuf]
		}
		for _, c := range out {
			buffer[ofs+inBuf] = byte(c)
			inBuf += 1
		}
		// fmt.Printf("uniq: %s %v\n", out, string(buffer))
	}
	return
}

type preparePattern struct {
	to      int64
	pattern func(i int) string
}

func prepareTestFile(fname string, pps []preparePattern) {
	total := pps[len(pps)-1].to
	buffer := make([]byte, total)
	bufferPos := 0
	for _, pp := range pps {
		inBuf := 0
		relSize := int(pp.to) - bufferPos
		for i := 0; inBuf < int(relSize); i += 1 {
			out := pp.pattern(i)
			if inBuf+len(out) >= int(relSize) {
				out = out[:int(relSize)-inBuf]
			}
			for _, c := range out {
				buffer[bufferPos+inBuf] = byte(c)
				inBuf += 1
			}
			// fmt.Printf("uniq: %s %v\n", out, string(buffer))
		}
		bufferPos += inBuf
	}
	if len(buffer) != int(total) {
		log.Fatalf("buffer size is not correct %d != %d", len(buffer), total)
	}
	bufferPos = 0
	for _, pp := range pps {
		o := pp.pattern(0)
		startPattern := buffer[bufferPos : bufferPos+len(o)]
		if string(startPattern) != o {
			log.Fatalf("buffer not prepared %v %v", o, startPattern)
		}
		relSize := int(pp.to) - bufferPos
		bufferPos += relSize
	}
	os.WriteFile(fname, buffer, 0644)
}

func TestShiftPart(t *testing.T) {
	fid := uuid.New().String()
	beforeLocation := int64(4711)
	afterLocation := int64(1147)
	beforeFname := fmt.Sprintf("before-%s.test", fid)
	args := MakeShiftPartCmd(&ShiftPartArgs{
		FileName:     beforeFname,
		FromLocation: &beforeLocation,
		ToLocation:   &afterLocation,
	})
	defer os.Remove(beforeFname)
	total := int64(127 * 1123 * 997)
	prepareTestFile(beforeFname, []preparePattern{
		{
			to: *args.FromLocation,
			pattern: func(i int) string {
				return fmt.Sprintf("%c%d", "pre"[i%3], i)
			},
		},
		{
			to: total,
			pattern: func(i int) string {
				return fmt.Sprintf("%c%d", "POST"[i%4], i)
			},
		},
	})

	afterFname := fmt.Sprintf("after-%s.test", fid)
	defer os.Remove(afterFname)
	prepareTestFile(afterFname, []preparePattern{
		{
			to: *args.ToLocation,
			pattern: func(i int) string {
				return fmt.Sprintf("%c%d", "pre"[i%3], i)
			},
		},
		{
			to: total - (*args.FromLocation - *args.ToLocation),
			pattern: func(i int) string {
				return fmt.Sprintf("%c%d", "POST"[i%4], i)
			},
		},
		{
			to: total - (*args.FromLocation - *args.ToLocation) + 512,
			pattern: func(i int) string {
				return "0123456789ABCDEF"
			},
		},
		{
			to: total,
			pattern: func(i int) string {
				if i == 0 {
					return "942"
				}
				return fmt.Sprintf("%c%d", "POST"[(i+2)%4], i+17032942)
			},
		},
	})

	if MainAction(args) != 0 {
		t.Error("exit code is not 0")
	}

	res, err := readercomp.FilesEqual(beforeFname, afterFname)
	if err != nil {
		t.Error(err)
	}
	if res == false {
		t.Error("files are not equal")
	}

}

func TestShiftPartSearchMarkerNotFound(t *testing.T) {
	fid := uuid.New().String()
	beforeLocation := int64(4711)
	afterLocation := int64(1147)
	beforeFname := fmt.Sprintf("before-%s.test", fid)
	searchMarker := true
	args := MakeShiftPartCmd(&ShiftPartArgs{
		FileName:     beforeFname,
		FromLocation: &beforeLocation,
		ToLocation:   &afterLocation,
		SearchMark:   &searchMarker,
	})
	defer os.Remove(beforeFname)
	total := int64(127 * 1123 * 997)
	prepareTestFile(beforeFname, []preparePattern{
		{
			to: total,
			pattern: func(i int) string {
				return "0123456789ABCDEFG"
			},
		},
	})

	if MainAction(args) == 0 {
		t.Error("exit code is 0")
	}

}

func TestShiftPartSearchMarkerFound(t *testing.T) {
	fid := uuid.New().String()
	beforeLocation := int64(4711)
	afterLocation := int64(1147)
	beforeFname := fmt.Sprintf("search-marker-found-%s.test", fid)
	_searchMarker := true
	args := MakeShiftPartCmd(&ShiftPartArgs{
		FileName:     beforeFname,
		FromLocation: &beforeLocation,
		ToLocation:   &afterLocation,
		SearchMark:   &_searchMarker,
	})
	defer os.Remove(beforeFname)
	total := int64(127 * 1123 * 997)
	markPos := total - ((total / 29) * 7)
	prepareTestFile(beforeFname, []preparePattern{
		{
			to: markPos,
			pattern: func(i int) string {
				return "0123456789abcdefg"
			},
		},
		{
			to: markPos + 511,
			pattern: func(i int) string {
				return "0123456789ABCDEF"
			},
		},
		{
			to: markPos + 511 + 512,
			pattern: func(i int) string {
				return "0123456789ABCDEF"
			},
		},
		{ // between the gaps
			to: markPos + 511 + (beforeLocation - afterLocation),
			pattern: func(i int) string {
				return "FEDCBA987654321X"
			},
		},
		{
			to: total,
			pattern: func(i int) string {
				return "0123456789abcdefg"
			},
		},
	})

	diffOfs := *args.FromLocation - *args.ToLocation
	chstatus := make(chan status, *args.QueueSize)
	statusDone := make(chan bool, *args.QueueSize)
	go statusLoop(chstatus, statusDone)
	if !searchMarker(args, chstatus) {
		t.Error("exit code is 0")
	}
	chstatus <- status{readCount: int64(-1)}
	chstatus <- status{writeCount: int64(-1)}
	<-statusDone
	m := make([]byte, 1)
	{
		fd, _ := os.Open(beforeFname)
		fd.Seek(*args.ToLocation, 0)
		fd.Read(m)
		defer fd.Close()
	}
	if *args.ToLocation != markPos+511 {
		t.Errorf("Wrong to location: %d %d", *args.ToLocation, markPos+511)
	}
	if m[0] != '0' {
		t.Errorf("Wrong to Byte at location: %c %d %d", m[0], *args.ToLocation, markPos+511)
	}
	if *args.FromLocation != markPos+511+diffOfs {
		t.Errorf("Wrong from location: %d %d", *args.FromLocation, markPos+511+diffOfs)
	}
	{
		fd, _ := os.Open(beforeFname)
		fd.Seek(*args.FromLocation, 0)
		fd.Read(m)
		defer fd.Close()
	}
	if m[0] != '0' {
		t.Errorf("Wrong from Byte at location: %c %d %d", m[0], *args.FromLocation, markPos+511+diffOfs)
	}

}

func TestShiftPartSearchMarker(t *testing.T) {
	fid := uuid.New().String()
	beforeLocation := int64(4711)
	afterLocation := int64(1147)
	beforeFname := fmt.Sprintf("before-search-marker-%s.test", fid)
	searchMarker := true
	total := int64(127 * 1123 * 997)
	markPos := total - ((total / 29) * 7)
	searchOffset := int64(1123 * 997)
	args := MakeShiftPartCmd(&ShiftPartArgs{
		FileName:     beforeFname,
		FromLocation: &beforeLocation,
		ToLocation:   &afterLocation,
		SearchMark:   &searchMarker,
		SearchOffset: &searchOffset,
	})
	defer os.Remove(beforeFname)
	prepareTestFile(beforeFname, []preparePattern{
		{
			to: *args.ToLocation,
			pattern: func(i int) string {
				return fmt.Sprintf("%c%d", "pre"[i%3], i)
			},
		},
		{
			to: markPos,
			pattern: func(i int) string {
				return fmt.Sprintf("%c%d", "POST"[i%4], i)
			},
		},
		{
			to: markPos + 512,
			pattern: func(i int) string {
				return "0123456789ABCDEF"
			},
		},
		{ // from to gap
			to: markPos + (beforeLocation - afterLocation),
			pattern: func(i int) string {
				return "FEDCBA987654321X"
			},
		},
		{
			to: total + (beforeLocation - afterLocation),
			pattern: func(i int) string {
				if i == 0 {
					return "3220069"
				}
				i += 13220069
				return fmt.Sprintf("%c%d", "POST"[i%4], i)
			},
		},
	})

	afterFname := fmt.Sprintf("after-search-marker-%s.test", fid)
	defer os.Remove(afterFname)
	prepareTestFile(afterFname, []preparePattern{
		{
			to: *args.ToLocation,
			pattern: func(i int) string {
				return fmt.Sprintf("%c%d", "pre"[i%3], i)
			},
		},
		{
			to: total,
			pattern: func(i int) string {
				return fmt.Sprintf("%c%d", "POST"[i%4], i)
			},
		},
	})

	if MainAction(args) != 0 {
		t.Error("exit code is not 0")
	}
	{
		err := os.Truncate(beforeFname, total)
		if err != nil {
			t.Error(err)
		}
	}
	// t.Errorf("%d %d %d", *args.FromLocation, *args.ToLocation, total)

	res, err := readercomp.FilesEqual(beforeFname, afterFname)
	if err != nil {
		t.Error(err)
	}
	if res == false {
		t.Error("files are not equal")
	}

}
