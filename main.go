package main

import (
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type ShiftPartArgs struct {
	FileName     string
	FromLocation *int64
	ToLocation   *int64
	BufferSize   *int64
	QueueSize    *int
	MarkerSize   *int
	SearchMark   *bool
	SearchOffset *int64

	Version   string
	GitCommit string
}

type buffer struct {
	len     int
	fileOfs int64
	buffer  []byte
}

type status struct {
	readError  error
	readCount  int64
	readtime   time.Duration
	readCalls  int64
	readOfs    int64
	readDone   bool
	writeError error
	writeCount int64
	writetime  time.Duration
	writeCalls int64
	writeDone  bool
	writeOfs   int64
}

func versionStr(args *ShiftPartArgs) string {
	return fmt.Sprintf("Version: %s:%s\n", args.Version, args.GitCommit)
}

func min(x, y int64) int64 {
	if x < y {
		return x
	}
	return y
}

func createMarker(spa *ShiftPartArgs) []byte {
	mark := make([]byte, min(*spa.FromLocation-*spa.ToLocation, int64(*spa.MarkerSize)))
	for i := range mark {
		mark[i] = byte("0123456789ABCDEF"[i%16])
	}
	return mark
}

func fileWriter(spa *ShiftPartArgs, chbuf chan buffer, chstatus chan status) {
	mark := createMarker(spa)

	fd, err := os.OpenFile(spa.FileName, os.O_WRONLY, 0644)
	if err != nil {
		chstatus <- status{readCount: int64(-1), writeError: err}
		return
	}
	defer fd.Close()

	// writer := bufio.NewWriter(fd)
	for buffer := range chbuf {
		if buffer.len == 0 {
			break
		}
		if buffer.fileOfs < *spa.ToLocation {
			chstatus <- status{readCount: int64(-1), writeError: fmt.Errorf("fileOfs < ToLocation")}
			return
		}
		buffer.buffer = append(buffer.buffer[:buffer.len], mark...)
		if len(buffer.buffer) != buffer.len+len(mark) {
			chstatus <- status{readCount: int64(-1), writeError: fmt.Errorf("buffer size is not correct")}
			return
		}
		toOfs := buffer.fileOfs - (*spa.FromLocation - *spa.ToLocation)
		// log.Printf("write %d bytes at %d", buffer.len, toOfs)
		start := time.Now()
		fd.Seek(toOfs, 0)
		count, err := fd.Write(buffer.buffer)
		// fd.Seek(int64(-len(mark)), 1)
		writetime := time.Since(start)
		if err != nil {
			chstatus <- status{readCount: int64(-1), writeError: err}
			return
		}
		chstatus <- status{writeCount: int64(count), writetime: writetime, writeOfs: toOfs}
	}
	chstatus <- status{writeCount: int64(-1)}
}

func fileReader(spa *ShiftPartArgs, chbuf chan buffer, chstatus chan status) {
	fd, err := os.Open(spa.FileName)
	if err != nil {
		chstatus <- status{readCount: int64(-1), readError: err}
		return
	}
	defer fd.Close()
	fd.Seek(*spa.FromLocation, 0)

	fileOfs := *spa.FromLocation
	// reader := bufio.NewReader(fd)
	for {
		ibuf := buffer{
			len:     0,
			fileOfs: fileOfs,
			buffer:  make([]byte, *spa.BufferSize, *spa.BufferSize+min(*spa.FromLocation-*spa.ToLocation, int64(*spa.MarkerSize))),
		}
		start := time.Now()
		count, err := fd.Read(ibuf.buffer)
		readtime := time.Since(start)
		if count == 0 {
			chbuf <- buffer{len: 0}
			chstatus <- status{readCount: int64(-1)}
			return
		}
		if err != nil {
			chstatus <- status{readCount: int64(-1), readError: err}
			return
		}
		if count > 0 {
			fileOfs += int64(count)
			chstatus <- status{readCount: int64(count), readtime: readtime, readOfs: fileOfs}
			ibuf.len = count
			chbuf <- ibuf
			continue
		}
	}

}

func versionCmd(arg *ShiftPartArgs) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "version",
		Long:  strings.TrimSpace(`print version`),
		Args:  cobra.MinimumNArgs(0),
		RunE: func(*cobra.Command, []string) error {
			fmt.Printf("Version: %s:%s\n", arg.Version, arg.GitCommit)
			return nil
		},
	}
}

func MakeShiftPartCmd(spa *ShiftPartArgs) *ShiftPartArgs {
	defaultBufferSize := int64(1024 * 1024)
	if spa.BufferSize == nil {
		spa.BufferSize = &defaultBufferSize
	}
	defaultQueueSize := int(16)
	if spa.QueueSize == nil {
		spa.QueueSize = &defaultQueueSize
	}
	defaultMarkerSize := int(512)
	if spa.MarkerSize == nil {
		spa.MarkerSize = &defaultMarkerSize
	}
	defaultSearchMark := false
	if spa.SearchMark == nil {
		spa.SearchMark = &defaultSearchMark
	}
	defaultSearchOffset := int64(0)
	if spa.SearchOffset == nil {
		spa.SearchOffset = &defaultSearchOffset
	}
	return spa
}

func main() {
	args := MakeShiftPartCmd(&ShiftPartArgs{})
	// _, err := buildArgs(os.Args, &args)
	rootCmd := &cobra.Command{
		Use: path.Base(os.Args[0]),
		// 	Name:       "neckless",
		// 	ShortUsage: "neckless subcommand [flags]",
		Short:   "shiftpart short help",
		Long:    strings.TrimSpace("shiftpart long help"),
		Version: versionStr(args),
		Args:    cobra.MinimumNArgs(0),
		// RunE:         gpgRunE(args),
		SilenceUsage: true,
	}
	// rootCmd.SetOut(args.Nio.out.first().writer())
	// rootCmd.SetErr(args.Nio.err.first().writer())
	rootCmd.SetArgs(os.Args[1:])

	flags := rootCmd.PersistentFlags()
	flags.StringVar(&args.FileName, "filename", "", "filename to shift")
	args.FromLocation = flags.Int64("from", -1, "from location")
	args.ToLocation = flags.Int64("to", -1, "to location")
	args.BufferSize = flags.Int64("buffer", *args.BufferSize, "buffer size")
	args.QueueSize = flags.Int("qsize", *args.QueueSize, "queue size")
	args.SearchMark = flags.Bool("searchMark", false, "search mark")
	args.SearchOffset = flags.Int64("searchOffset", 0, "search Offset")

	err := rootCmd.Execute()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if args.FileName != "" {
		fmt.Println("we need a filename ")
	}
	if *args.FromLocation > 0 {
		fmt.Println("we need a fromlocation ")
	}
	if *args.ToLocation > 0 {
		fmt.Println("we need a tolocation ")
	}

	if *args.ToLocation <= *args.FromLocation {
		fmt.Println("to location must be greater than from location")
	}
	fmt.Println("filename", args.FileName)
	fmt.Println("from", *args.FromLocation)
	fmt.Println("to", *args.ToLocation)
	fmt.Println("buffer", *args.BufferSize)
	fmt.Println("qsize", *args.QueueSize)

	os.Exit(MainAction(args))
}

func searchMarker(spa *ShiftPartArgs, chstatus chan status) bool {
	if !*spa.SearchMark {
		return true
	}
	log.Println("started search marker")
	mark := createMarker(spa)
	fd, err := os.Open(spa.FileName)
	if err != nil {
		chstatus <- status{readCount: int64(-1), readError: err}
		return false
	}
	defer fd.Close()
	fd.Seek(*spa.SearchOffset, 0)
	// reader := bufio.NewReaderSize(fd, int(*spa.BufferSize))
	possibleMarkerOfs := 0
	filePos := *spa.SearchOffset
	log.Printf("search marker at %d:%d to:%d from:%d\n", filePos, len(mark), *spa.ToLocation, *spa.FromLocation)
	readBuffer := make([]byte, *spa.BufferSize)
	for {
		start := time.Now()
		count, err := fd.Read(readBuffer)
		readtime := time.Since(start)
		if count == 0 {
			chstatus <- status{readCount: int64(-1), writeCount: int64(-1)}
			return false
		}
		if err != nil {
			chstatus <- status{readCount: int64(-1), writeCount: int64(-1), readError: err}
			return false
		}
		chstatus <- status{readCount: int64(count), readtime: readtime}
		for _, val := range readBuffer[:count] {
			for i := 0; i < 2; i++ {
				if mark[possibleMarkerOfs] == val {
					possibleMarkerOfs += 1
					if possibleMarkerOfs == len(mark) {
						markpos := (filePos + 1) - int64(len(mark))
						fromLocation := markpos + (*spa.FromLocation - *spa.ToLocation)
						spa.FromLocation = &fromLocation
						spa.ToLocation = &markpos
						log.Printf("found search marker: filePos:%d newTo: %d newFrom: %d", filePos, *spa.ToLocation, *spa.FromLocation)
						return true
					}
					break
				} else {
					possibleMarkerOfs = 0
				}
			}
			filePos += 1
		}
	}
}

func statusLoop(chstatus chan status, statusDone chan bool) {
	go func() {
		log.Println("started status")

		chlogDone := make(chan bool)
		total := status{}
		go func() {
			log.Println("started status output")
			for {
				log.Printf("%v read: ofs:%d %d(%d-%vs) write: ofs:%d %d(%d-%vs)%v\n",
					time.Now(),
					total.readOfs,
					total.readCount, total.readCalls, total.readtime.Seconds(),
					total.writeOfs,
					total.writeCount, total.writeCalls, total.writetime.Seconds(), total.readDone && total.writeDone)
				if total.writeDone && total.readDone {
					chlogDone <- true
					break
				}
				time.Sleep(time.Second)
			}
			log.Println("done status output")
		}()

		for ch := range chstatus {
			total.readCount += ch.readCount
			total.readtime += ch.readtime
			if ch.readOfs > 0 {
				total.readOfs = ch.readOfs
			}
			if ch.readCount > 0 {
				total.readCalls += 1
			}
			total.writeCount += ch.writeCount
			total.writetime += ch.writetime
			if ch.writeOfs > 0 {
				total.writeOfs = ch.writeOfs
			}
			if ch.writeCount > 0 {
				total.writeCalls += 1
			}
			if ch.writeCount == -1 {
				total.writeDone = true
			}
			if ch.readCount == -1 {
				total.readDone = true
			}
			if ch.readError != nil {
				log.Fatalf("ReadError %v\n", ch.readError)
			}
			if ch.writeError != nil {
				log.Fatalf("WriteError %v\n", ch.writeError)
			}

			if total.writeDone && total.readDone {
				break
			}
		}
		<-chlogDone
		log.Println("status stopped")
		statusDone <- true
	}()
}

func MainAction(args *ShiftPartArgs) int {
	statusDone := make(chan bool)
	chbuf := make(chan buffer, *args.QueueSize)
	chstatus := make(chan status, *args.QueueSize)

	go statusLoop(chstatus, statusDone)

	ret := 0
	if searchMarker(args, chstatus) {
		go fileWriter(args, chbuf, chstatus)
		fileReader(args, chbuf, chstatus)
	} else {
		log.Printf("search marker not found")
		ret = 1
	}
	<-statusDone
	log.Println("done")
	return ret
}
