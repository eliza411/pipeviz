package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	gjs "github.com/tag1consulting/pipeviz/Godeps/_workspace/src/github.com/xeipuuv/gojsonschema"
	"github.com/tag1consulting/pipeviz/Godeps/_workspace/src/github.com/zenazn/goji/graceful"
	"github.com/tag1consulting/pipeviz/Godeps/_workspace/src/github.com/zenazn/goji/web"
	"github.com/tag1consulting/pipeviz/Godeps/_workspace/src/github.com/zenazn/goji/web/middleware"
	"github.com/tag1consulting/pipeviz/broker"
	"github.com/tag1consulting/pipeviz/interpret"
	"github.com/tag1consulting/pipeviz/persist"
	"github.com/tag1consulting/pipeviz/represent"
	"github.com/tag1consulting/pipeviz/webapp"
)

type message struct {
	Id  int
	Raw []byte
}

// Pipeviz has two fully separated HTTP ports - one for input into the logic
// machine, and one for graph data consumption. This is done primarily
// because security/firewall concerns are completely different, and having
// separate ports makes it much easier to implement separate policies.
// Differing semantics are a contributing, but lesser consideration.
const (
	DefaultIngestionPort = 2309 // 2309, because Cayte
	DefaultAppPort       = 8008
	MaxMessageSize       = 5 << 20 // Max input message size is 5MB
)

func main() {
	src, err := ioutil.ReadFile("./schema.json")
	if err != nil {
		panic(err.Error())
	}

	// The master JSON schema used for validating all incoming messages
	masterSchema, err := gjs.NewSchema(gjs.NewStringLoader(string(src)))
	if err != nil {
		panic(err.Error())
	}

	// Channel to receive persisted messages from HTTP workers. 1000 cap to allow
	// some wiggle room if there's a sudden burst of messages and the interpreter
	// gets behind.
	interpretChan := make(chan message, 1000)

	// Kick off the http message ingestor.
	// TODO let config/params control address
	go RunHttpIngestor("127.0.0.1:"+strconv.Itoa(DefaultIngestionPort), masterSchema, interpretChan)

	// Kick off fanout on the master/singleton graph broker. This will bridge between
	// the state machine and the listeners interested in the machine's state.
	brokerChan := make(chan represent.CoreGraph, 0)
	broker.Get().Fanout(brokerChan)

	// Kick off the intermediary interpretation goroutine that receives persisted
	// messages from the ingestor, merges them into the state graph, then passes
	// them along to the graph broker.
	go Interpret(represent.NewGraph(), interpretChan, brokerChan) // for now, always a new graph

	// And finally, kick off the webapp.
	// TODO let config/params control address
	go RunWebapp("127.0.0.1:" + strconv.Itoa(DefaultAppPort))

	// Block on goji's graceful waiter, allowing the http connections to shut down nicely.
	// FIXME using this should be unnecessary if we're crash-only
	graceful.Wait()
}

// RunHttpIngestor sets up and runs the http listener that receives messages, validates
// them against the provided schema, persists those that pass validation, then sends
// them along to the interpretation layer via the provided channel.
//
// This blocks on the http listening loop, so it should typically be called in its own goroutine.
//
// Closes the provided interpretation channel if/when the http server terminates.
func RunHttpIngestor(addr string, schema *gjs.Schema, ich chan<- message) {
	mb := web.New()
	// TODO use more appropriate logger
	mb.Use(middleware.Logger)
	// Add middleware limiting body length to MaxMessageSize
	mb.Use(func(h http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, MaxMessageSize)
			h.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	})

	mb.Post("/", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		// pulling this out of buffer is incorrect: http://jmoiron.net/blog/crossing-streams-a-love-letter-to-ioreader/
		// but for now gojsonschema leaves us no choice
		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			// Too long, or otherwise malformed request body
			// TODO add a body
			w.WriteHeader(400)
			w.Write([]byte(err.Error()))
			return
		}

		result, err := schema.Validate(gjs.NewStringLoader(string(b)))
		if err != nil {
			// Malformed JSON, likely
			// TODO add a body
			w.WriteHeader(400)
			w.Write([]byte(err.Error()))
			return
		}

		if result.Valid() {
			id := persist.Append(b, r.RemoteAddr)

			// super-sloppy write back to client, but does the trick
			w.WriteHeader(202) // use 202 because it's a little more correct
			w.Write([]byte(strconv.Itoa(id)))

			// FIXME passing directly from here means it's possible for messages to arrive
			// at the interpretation layer in a different order than they went into the log
			// ...especially if go scheduler changes become less cooperative https://groups.google.com/forum/#!topic/golang-nuts/DbmqfDlAR0U (...?)

			ich <- message{Id: id, Raw: b}
		} else {
			// Invalid results, so write back 422 for malformed entity
			w.WriteHeader(422)
			var resp []string
			for _, desc := range result.Errors() {
				resp = append(resp, desc.String())
			}
			w.Write([]byte(strings.Join(resp, "\n")))
		}
	})

	graceful.ListenAndServe(addr, mb)
	close(ich)
}

// The main message interpret/merge loop. This receives messages that have been
// validated and persisted, merges them into the graph, then sends the new
// graph along to listeners, workers, etc.
//
// The provided CoreGraph operates as the initial state into which received
// messages will be successively merged.
//
// When the interpret channel is closed (and emptied), this function also closes
// the broker channel.
func Interpret(g represent.CoreGraph, ich <-chan message, bch chan<- represent.CoreGraph) {
	for m := range ich {
		// TODO msgid here should be strictly sequential; check, and add error handling if not
		im := interpret.Message{Id: m.Id}
		json.Unmarshal(m.Raw, &im)
		g = g.Merge(im)

		bch <- g
	}
	close(bch)
}

// RunWebapp runs the pipeviz http frontend webapp on the provided address.
//
// This blocks on the http listening loop, so it should typically be called in its own goroutine.
func RunWebapp(addr string) {
	mf := webapp.NewMux()
	graceful.ListenAndServe(addr, mf)
}
