package component

import (
	"encoding/json"
	"expvar"
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"

	"github.com/mackerelio/go-osstat/cpu"
	"github.com/mackerelio/go-osstat/memory"

	nats "github.com/nats-io/go-nats"
	"github.com/nats-io/nuid"
)

// Component - base component
type Component struct {
	kind        string
	uuid        string
	nc          *nats.Conn
	mu          *sync.Mutex
	systemTopic string
	started     time.Time
}

// NewComponent - creates new component
func NewComponent(kind string) *Component {
	return &Component{
		kind:    kind,
		uuid:    nuid.Next(),
		mu:      &sync.Mutex{},
		started: time.Now(),
	}
}

// SetupConnectionToNATS - creates connection
func (c *Component) SetupConnectionToNATS(servers, systemTopic string, options ...nats.Option) (err error) {
	options = append(options, nats.Name(c.Name()))
	c.systemTopic = systemTopic
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nc, err = nats.Connect(servers, options...)
	if err != nil {
		return
	}

	c.setNATSEventHandlers()
	c.discovery()

	return
}

// sets handlers for live cicle of component for diagnostics
func (c *Component) setNATSEventHandlers() {
	c.nc.SetErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) {
		log.Printf("NATS Error: %s\n", err)
	})

	c.nc.SetReconnectHandler(func(_ *nats.Conn) {
		log.Println("Reconnected!")
	})

	c.nc.SetDisconnectHandler(func(_ *nats.Conn) {
		log.Println("Disconnected!")
	})

	c.nc.SetClosedHandler(func(_ *nats.Conn) {
		log.Println("Connection to NATS is closed!")
	})
}

// subsribe to main topics for component diagnostics
func (c *Component) discovery() (err error) {
	if err = c.discoveryInfo(); err != nil {
		log.Println(err)
	}

	if err = c.discoveryStatus(); err != nil {
		log.Println(err)
	}

	return
}

// dicsoveryInfo - subscribes to discovery topic and reply with main component info
func (c *Component) discoveryInfo() (err error) {
	_, err = c.nc.Subscribe(fmt.Sprintf("_%s.discovery", c.systemTopic), func(m *nats.Msg) {
		if m.Reply != "" {
			response := Info{
				ID:      c.ID(),
				Kind:    c.kind,
				Started: c.Started(),
			}
			jsonResponse, err := json.Marshal(response)
			if err != nil {
				return
			}
			c.nc.Publish(m.Reply, jsonResponse)
		}
	})

	return
}

// discoveryStatus - send to NATS server component stats
func (c *Component) discoveryStatus() (err error) {
	_, err = c.nc.Subscribe(fmt.Sprintf("_%s.%s.status", c.systemTopic, c.uuid), func(m *nats.Msg) {
		if m.Reply != "" {
			statsz := Stats{
				Kind:     c.kind,
				ID:       c.uuid,
				Cmd:      expvar.Get("cmdline").(expvar.Func)().([]string),
				MemStats: expvar.Get("memstats").(expvar.Func)().(runtime.MemStats),
			}
			mem, err := memory.Get()
			if err == nil {
				statsz.System = SystemStats{
					MemoryFree:  mem.Free,
					MemoryTotal: mem.Total,
					MemoryUsed:  mem.Used,
				}
			}
			CPU, err := cpu.Get()
			if err == nil {
				statsz.System.CPUSystem = CPU.System
				statsz.System.CPUTotal = CPU.Total
				statsz.System.CPUCount = CPU.CPUCount
			}

			result, err := json.Marshal(statsz)
			if err != nil {
				log.Println(err)
				return
			}
			log.Println(statsz)
			c.nc.Publish(m.Reply, result)
		}
	})

	return
}

// Name - component name
func (c *Component) Name() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return fmt.Sprintf("%s:%s", c.kind, c.uuid)
}

// NATS - component name
func (c *Component) NATS() *nats.Conn {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.nc
}

// ID - component ID
func (c *Component) ID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.uuid
}

// SystemTopic - system topic
func (c *Component) SystemTopic() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.systemTopic
}

// Started - time when component connects to NATS server
func (c *Component) Started() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.started
}

// Shutdown - close nats connection
func (c *Component) Shutdown() error {
	c.NATS().Close()
	return nil
}
