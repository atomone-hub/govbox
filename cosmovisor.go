package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/go-github/v24/github"
	"github.com/peterbourgon/ff/v3/ffcli"
)

func init() {
	rootCmd.Subcommands = append(rootCmd.Subcommands, cosmovisorBinInfoCmd())
}

func cosmovisorBinInfoCmd() *ffcli.Command {
	fs := flag.NewFlagSet("cosmovisorBinInfo", flag.ContinueOnError)
	return &ffcli.Command{
		Name:       "cosmovisor-bin-info",
		ShortUsage: "govbox cosmovisor-bin-info TAG",
		ShortHelp:  "Generate the JSON expected by cosmovisor to automatically fetch the binaries",
		Exec: func(ctx context.Context, args []string) error {
			if err := fs.Parse(args); err != nil {
				return err
			}
			if fs.NArg() != 1 {
				return flag.ErrHelp
			}

			// fetch release assets
			client := github.NewClient(nil)
			const (
				owner = "atomone-hub"
				repo  = "atomone"
			)
			tag := args[0]
			rr, _, err := client.Repositories.GetReleaseByTag(context.Background(), owner, repo, tag)
			if err != nil {
				return err
			}
			var (
				expectedPrefix = fmt.Sprintf("atomoned-%s-", tag)
				binaries       = make(map[string]string)
				checksumFile   = fmt.Sprintf("SHA256SUMS-%s.txt", tag)
				checksums      = make(map[string]string)
			)
			// loop on assets to fill the checksums
			for _, a := range rr.Assets {
				if a.GetName() != checksumFile {
					continue
				}
				// fetch checksum file
				resp, err := http.Get(a.GetBrowserDownloadURL())
				// r, _, err := client.Repositories.DownloadReleaseAsset(context.Background(), owner, repo, a.GetID())
				if err != nil {
					return err
				}
				defer resp.Body.Close()
				r := bufio.NewReader(resp.Body)
				for {
					line, _, err := r.ReadLine()
					if err != nil {
						if err == io.EOF {
							break
						}
						return err
					}
					// parse line
					var (
						checksum string
						binary   string
					)
					_, _ = fmt.Sscanf(string(line), "%s %s", &checksum, &binary)
					checksums[binary] = checksum
				}
				break
			}
			// loop on binary assets
			for _, a := range rr.Assets {
				if strings.HasPrefix(expectedPrefix, a.GetName()) {
					continue
				}
				value := a.GetBrowserDownloadURL() + "?checksum=sha256:" + checksums[a.GetName()]
				switch a.GetName()[len(expectedPrefix):] {
				case "darwin-amd64":
					binaries["darwin/amd64"] = value
				case "linux-amd64":
					binaries["linux/amd64"] = value
				case "darwin-arm64":
					binaries["darwin/arm64"] = value
				case "linux-arm64":
					binaries["linux/arm64"] = value
				}
			}
			bz, err := json.Marshal(map[string]any{"binaries": binaries})
			if err != nil {
				return err
			}
			fmt.Println(strconv.Quote(string(bz)))
			return nil
		},
	}
}
