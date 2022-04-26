package main

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/spf13/cobra"
)

type ShiftPartArgs struct {
	FileName     *string
	FromLocation *int64
	ToLocation   *int64

	Version   string
	GitCommit string
}

func versionStr(args *ShiftPartArgs) string {
	return fmt.Sprintf("Version: %s:%s\n", args.Version, args.GitCommit)
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

func main() {
	args := ShiftPartArgs{
		FileName:     nil,
		FromLocation: nil,
		ToLocation:   nil,
	}
	// _, err := buildArgs(os.Args, &args)
	rootCmd := &cobra.Command{
		Use: path.Base(os.Args[0]),
		// 	Name:       "neckless",
		// 	ShortUsage: "neckless subcommand [flags]",
		Short:   "neckless short help",
		Long:    strings.TrimSpace("neckless long help"),
		Version: versionStr(&args),
		Args:    cobra.MinimumNArgs(0),
		// RunE:         gpgRunE(args),
		SilenceUsage: true,
	}
	// rootCmd.SetOut(args.Nio.out.first().writer())
	// rootCmd.SetErr(args.Nio.err.first().writer())
	rootCmd.SetArgs(os.Args[1:])

	flags := rootCmd.PersistentFlags()
	flags.StringVar(args.FileName, "filename", "", "filename to shift")
	args.FromLocation = flags.Int64("from", -1, "from location")
	args.ToLocation = flags.Int64("to", -1, "to location")

	err := rootCmd.Execute()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("filename", *args.FileName)
	fmt.Println("to", *args.FromLocation)
	fmt.Println("from", *args.ToLocation)

}
