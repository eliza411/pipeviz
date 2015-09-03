package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	gjs "github.com/tag1consulting/pipeviz/Godeps/_workspace/src/github.com/xeipuuv/gojsonschema"
	"github.com/tag1consulting/pipeviz/interpret"
	"github.com/tag1consulting/pipeviz/schema"
)

var schemaMaster *gjs.Schema

// TODO we just use this as a way to namespace common names
type envCmd struct{}

func envCommand() *cobra.Command {
	ec := envCmd{}
	cmd := &cobra.Command{
		Use:   "env [-n|--no-detect]",
		Short: "Generates a pipeviz message describing an environment.",
		Long:  "Generates a valid pipeviz message describing an environment. Sends the message to a target server, if one is provided; otherwise prints the message and exits.",
		Run:   ec.runGenEnv,
	}

	var nodetect bool
	cmd.Flags().BoolVarP(&nodetect, "no-detect", "n", false, "Skip automated detection of suggested values.")

	return cmd
}

// runGenEnv is the main entry point for running the environment-generating
// env subcommand.
func (ec envCmd) runGenEnv(cmd *cobra.Command, args []string) {
	e := &interpret.Environment{}

	if !cmd.Flags().Lookup("no-detect").Changed {
		*e = detectEnvDefaults()
	}

	// Write directly to stdout, at least for now
	w := os.Stdout

	// Prep schema to validate the messages as we go
	raw, err := schema.Master()
	if err != nil {
		fmt.Fprintln(w, "WARNING: Failed to open master schema file; pvc cannot validate outgoing messages.")
	}

	schemaMaster, err = gjs.NewSchema(gjs.NewStringLoader(string(raw)))
	if err != nil {
		panic("bad schema...?")
	}

	client := http.Client{Timeout: 5 * time.Second}

	fmt.Fprintln(w, "Generating an environment message...")
	reader := bufio.NewReader(os.Stdin)
MenuLoop:
	for {
		fmt.Fprintf(w, "\n")
		ec.printCurrentState(w, *e)
		fmt.Fprintf(w, "\n")
		fmt.Fprintf(w, "Select a value to edit by number, (s)end, or (q)uit: ")

		var input string
		for {
			l, err := fmt.Fscanln(reader, &input)
			if l > 1 || err != nil {
				continue
			}

			switch input {
			case "q", "quit":
				fmt.Fprintf(w, "\nQuitting; message was not sent\n")
				os.Exit(1)
			case "s", "send":
				// wrap the env up in a map that'll marshal into workable JSON
				m := wrapForJSON(*e)

				msg, err := json.Marshal(m)
				if err != nil {
					log.Fatalf("\nFailed to marshal JSON of environment object, no message sent\n")
				}

				resp, err := client.Post(cmd.Flags().Lookup("target").Value.String(), "application/json", bytes.NewReader(msg))
				if err != nil {
					log.Fatalf(err.Error())
				}

				bod, err := ioutil.ReadAll(resp.Body)
				resp.Body.Close()
				if err != nil {
					log.Fatalf(err.Error())
				}

				if resp.StatusCode >= 200 && resp.StatusCode <= 300 {
					fmt.Printf("Message accepted (%v), msgid %v\n", resp.StatusCode, string(bod))
				} else {
					fmt.Printf("Message was rejected with code %v and message %v\n", resp.StatusCode, string(bod))
				}
				break MenuLoop

			default:
				num, interr := strconv.Atoi(input)
				if interr != nil {
					continue
				} else if 0 < num && num < 7 {
					switch num {
					case 1:
						collectFQDN(w, reader, e)
					case 2:
						collectIpv4(w, reader, e)
					case 3:
						collectIpv6(w, reader, e)
					case 4:
						collectOS(w, reader, e)
					case 5:
						collectNick(w, reader, e)
					case 6:
						collectProvider(w, reader, e)
					}
					continue MenuLoop
				} else {
					continue
				}
			}
		}
	}
}

// wrapForJSON converts an environment into a map that will serialize
// appropriate pipeviz message JSON.
func wrapForJSON(e interpret.Environment) map[string]interface{} {
	m := make(map[string]interface{})
	m["environments"] = []interpret.Environment{e}
	return m
}

func collectFQDN(w io.Writer, r io.Reader, e *interpret.Environment) {
	fmt.Fprintf(w, "\n\nEditing FQDN\nCurrent Value: %q\n", e.Address.Hostname)
	fmt.Fprint(w, "New value: ")

	for {
		var input string
		_, err := fmt.Fscanln(r, &input)
		if err == nil {
			e.Address.Hostname = input
			break
		}

		fmt.Fprintf(w, "\nInvalid input.\nNew value: ")
	}
}

func collectIpv4(w io.Writer, r io.Reader, e *interpret.Environment) {
	fmt.Fprintf(w, "\n\nEditing IPv4\nCurrent Value: %q\n", e.Address.Ipv4)
	fmt.Fprint(w, "New value: ")

	for {
		var input string
		_, err := fmt.Fscanln(r, &input)
		if err == nil {
			addr := net.ParseIP(input)
			if addr == nil {
				// failed to parse IP, invalid input
				fmt.Fprintf(w, "\nNot a valid IP address.\nNew value: ")
			} else if addr.To4() == nil {
				// not a valid IPv4
				fmt.Fprintf(w, "\nNot a valid IPv4 address.\nNew value: ")
			} else {
				e.Address.Ipv4 = addr.String()
				break
			}
		} else {
			fmt.Fprintf(w, "\nInvalid input.\nNew value: ")
		}
	}
}

func collectIpv6(w io.Writer, r io.Reader, e *interpret.Environment) {
	fmt.Fprintf(w, "\n\nEditing IPv6\nCurrent Value: %q\n", e.Address.Ipv6)
	fmt.Fprint(w, "New value: ")

	for {
		var input string
		_, err := fmt.Fscanln(r, &input)
		if err == nil {
			addr := net.ParseIP(input)
			if addr == nil {
				// failed to parse IP, invalid input
				fmt.Fprintf(w, "\nNot a valid IP address.\nNew value: ")
			} else if addr.To16() == nil {
				// not a valid IPv4
				fmt.Fprintf(w, "\nNot a valid IPv6 address.\nNew value: ")
			} else {
				e.Address.Ipv6 = addr.String()
				break
			}
		} else {
			fmt.Fprintf(w, "\nInvalid input.\nNew value: ")
		}
	}
}

func collectOS(w io.Writer, r io.Reader, e *interpret.Environment) {
	fmt.Fprintf(w, "\n\nEditing OS\nCurrent Value: %q\n", e.OS)
	fmt.Fprint(w, "New value: ")

	for {
		var input string
		_, err := fmt.Fscanln(r, &input)
		if err == nil {
			e.OS = input
			break
		}

		fmt.Fprintf(w, "\nInvalid input.\nNew value: ")
	}
}

func collectNick(w io.Writer, r io.Reader, e *interpret.Environment) {
	fmt.Fprintf(w, "\n\nEditing Nick\nCurrent Value: %q\n", e.Nick)
	fmt.Fprint(w, "New value: ")

	for {
		var input string
		_, err := fmt.Fscanln(r, &input)
		if err == nil {
			e.Nick = input
			break
		}

		fmt.Fprintf(w, "\nInvalid input.\nNew value: ")
	}
}

func collectProvider(w io.Writer, r io.Reader, e *interpret.Environment) {
	fmt.Fprintf(w, "\n\nEditing Provider\nCurrent Value: %q\n", e.Provider)
	fmt.Fprint(w, "New value: ")

	for {
		var input string
		_, err := fmt.Fscanln(r, &input)
		if err == nil {
			e.Provider = input
			break
		}

		fmt.Fprintf(w, "\nInvalid input.\nNew value: ")
	}
}

// Inspects the currently running system to fill in some default values.
func detectEnvDefaults() (e interpret.Environment) {
	var err error
	e.Address.Hostname, err = os.Hostname()
	if err != nil {
		e.Address.Hostname = ""
	}

	e.OS = runtime.GOOS

	return e
}

// printMenu prints to stdout a menu showing the current data in the
// message to be generated.
func (ec envCmd) printCurrentState(w io.Writer, e interpret.Environment) {
	fmt.Fprintln(w, "Environment data:")
	var n int

	n++
	//if e.Address.Hostname == "" {
	//fmt.Fprintf(w, "  %v. *FQDN: [empty]\n", n)
	//} else {
	fmt.Fprintf(w, "  %v. FQDN: %q\n", n, e.Address.Hostname)
	//}

	n++
	fmt.Fprintf(w, "  %v. Ipv4: %q\n", n, e.Address.Ipv4)
	n++
	fmt.Fprintf(w, "  %v. Ipv6: %q\n", n, e.Address.Ipv6)

	n++
	fmt.Fprintf(w, "  %v. OS: %q\n", n, e.OS)

	n++
	fmt.Fprintf(w, "  %v. Nick: %q\n", n, e.Nick)

	n++
	fmt.Fprintf(w, "  %v. Provider: %q\n", n, e.Nick)

	validateAndPrint(w, e)
}

func validateAndPrint(w io.Writer, e interpret.Environment) {
	// Convert the env to JSON
	m := wrapForJSON(e)

	msg, err := json.Marshal(m)
	if err != nil {
		fmt.Fprintf(w, "\nError while marshaling data to JSON for validation: %s\n", err.Error())
		return
	}

	// Validate the current state of the message
	result, err := schemaMaster.Validate(gjs.NewStringLoader(string(msg)))
	if err != nil {
		fmt.Fprintf(w, "\nError while attempting to validate data: %s\n", err.Error())
		return
	}
	if !result.Valid() {
		fmt.Fprintln(w, "\nAs it stands now, the data will fail validation if sent to a pipeviz server. Errors:")
		for _, desc := range result.Errors() {
			fmt.Fprintf(w, "\t%s\n", desc)
		}
	}
}
