package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	log "github.com/tag1consulting/pipeviz/Godeps/_workspace/src/github.com/Sirupsen/logrus"
	"github.com/tag1consulting/pipeviz/Godeps/_workspace/src/github.com/spf13/pflag"
	gjs "github.com/tag1consulting/pipeviz/Godeps/_workspace/src/github.com/xeipuuv/gojsonschema"
	"github.com/tag1consulting/pipeviz/Godeps/_workspace/src/github.com/zenazn/goji/graceful"
	"github.com/tag1consulting/pipeviz/Godeps/_workspace/src/github.com/zenazn/goji/web"
	"github.com/tag1consulting/pipeviz/broker"
	"github.com/tag1consulting/pipeviz/ingest"
	"github.com/tag1consulting/pipeviz/journal"
	"github.com/tag1consulting/pipeviz/journal/boltdb"
	"github.com/tag1consulting/pipeviz/represent"
	"github.com/tag1consulting/pipeviz/schema"
	"github.com/tag1consulting/pipeviz/types/system"
	"github.com/tag1consulting/pipeviz/webapp"
)

// Pipeviz uses two separate HTTP ports - one for input into the logic
// machine, and one for graph data consumption. This is done primarily
// because security/firewall concerns are completely different, and having
// separate ports makes it much easier to implement separate policies.
// Differing semantics are a contributing, but lesser consideration.
const (
	DefaultIngestionPort int = 2309 // 2309, because Cayte
	DefaultAppPort           = 8008
	MaxMessageSize           = 5 << 20 // Max input message size is 5MB
)

var (
	bindAll *bool   = pflag.BoolP("bind-all", "b", false, "Listen on all interfaces. Applies both to ingestor and webapp.")
	dbPath  *string = pflag.StringP("data-dir", "d", ".", "The base directory to use for persistent storage.")
)

func init() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:  true,
		DisableSorting: true,
	})
}

func main() {
	// If built with debug tag, turns on the http-based profiler. Otherwise, nop.
	runProfiler()

	src, err := schema.Master()
	if err != nil {
		log.WithFields(log.Fields{
			"system": "main",
			"err":    err,
		}).Fatal("Could not locate master schema file, exiting")
	}

	// The master JSON schema used for validating all incoming messages
	masterSchema, err := gjs.NewSchema(gjs.NewStringLoader(string(src)))
	if err != nil {
		log.WithFields(log.Fields{
			"system": "main",
			"err":    err,
		}).Fatal("Error while creating a schema object from the master schema file, exiting")
	}

	// Channel to receive persisted messages from HTTP workers. 1000 cap to allow
	// some wiggle room if there's a sudden burst of messages and the interpreter
	// gets behind.
	interpretChan := make(chan *journal.Record, 1000)

	pflag.Parse()
	var listenAt string
	if *bindAll == false {
		listenAt = "127.0.0.1:"
	} else {
		listenAt = ":"
	}

	j, err := boltdb.NewBoltStore(*dbPath + "/journal.bolt")
	if err != nil {
		log.WithFields(log.Fields{
			"system": "main",
			"err":    err,
		}).Fatal("Error while setting up journal store, exiting")
	}

	// Restore the graph from the journal (or start from nothing if journal is empty)
	// TODO move this down to after ingestor is started
	g, err := restoreGraph(j)
	if err != nil {
		log.WithFields(log.Fields{
			"system": "main",
			"err":    err,
		}).Fatal("Error while rebuilding the graph from the journal")
	}

	// Kick off fanout on the master/singleton graph broker. This will bridge between
	// the state machine and the listeners interested in the machine's state.
	brokerChan := make(chan system.CoreGraph, 0)
	broker.Get().Fanout(brokerChan)
	brokerChan <- g

	srv := ingest.New(j, masterSchema, interpretChan, brokerChan, MaxMessageSize)

	// Kick off the http message ingestor.
	// TODO let config/params control address
	go srv.RunHttpIngestor(listenAt + strconv.Itoa(DefaultIngestionPort))

	// Kick off the intermediary interpretation goroutine that receives persisted
	// messages from the ingestor, merges them into the state graph, then passes
	// them along to the graph broker.
	go srv.Interpret(g)

	// And finally, kick off the webapp.
	// TODO let config/params control address
	go RunWebapp(listenAt+strconv.Itoa(DefaultAppPort), j.Get)

	// Block on goji's graceful waiter, allowing the http connections to shut down nicely.
	// FIXME using this should be unnecessary if we're crash-only
	graceful.Wait()
}

// RunWebapp runs the pipeviz http frontend webapp on the specified address.
//
// This blocks on the http listening loop, so it should typically be called in its own goroutine.
func RunWebapp(addr string, f journal.RecordGetter) {
	mf := webapp.NewMux()

	// A middleware to attach the journal-getting func to the env for later use.
	mw := func(c *web.C, h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if c.Env == nil {
				c.Env = make(map[interface{}]interface{})
			}
			c.Env["journalGet"] = f
			h.ServeHTTP(w, r)
		})
	}

	mf.Use(mw)

	mf.Compile()
	graceful.ListenAndServe(addr, mf)
}

// Rebuilds the graph from the extant entries in a journal.
func restoreGraph(j journal.JournalStore) (system.CoreGraph, error) {
	g := represent.NewGraph()

	var item *journal.Record
	tot, err := j.Count()
	if err != nil {
		// journal failed to report a count for some reason, bail out
		return g, err
	} else if tot > 0 {
		// we manually iterate to the count because we assume that any messages
		// that come in while we do this processing will be queued elsewhere.
		for i := uint64(1); i-1 < tot; i++ {
			item, err = j.Get(i)
			if err != nil {
				// TODO returning out here could end us up somwehere weird
				return g, err
			}
			msg := &ingest.Message{}
			json.Unmarshal(item.Message, msg)
			g = g.Merge(item.Index, msg.UnificationForm())
		}
	}

	return g, nil
}
