package component

import (
	"encoding/json"
	"expvar"
	"fmt"
	"log"
	"runtime"
	"sync"

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
}

// NewComponent - creates new component
func NewComponent(kind string) *Component {
	return &Component{
		kind: kind,
		uuid: nuid.Next(),
		mu:   &sync.Mutex{},
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
	// send component UUID
	_, err = c.nc.Subscribe(fmt.Sprintf("_%s.discovery", c.systemTopic), func(m *nats.Msg) {
		if m.Reply != "" {
			response := Info{
				ID:   c.ID(),
				Kind: c.kind,
			}
			jsonResponse, err := json.Marshal(response)
			if err != nil {
				log.Println(err)
				return
			}
			c.nc.Publish(m.Reply, jsonResponse)
		}
	})
	// send component diagnostic info
	_, err = c.nc.Subscribe(fmt.Sprintf("_%s.%s.status", c.systemTopic, c.uuid), func(m *nats.Msg) {
		if m.Reply != "" {
			log.Println("[Status] Replying with status...")
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
				log.Printf("Error: %s\n", err)
				return
			}
			c.nc.Publish(m.Reply, result)
		} else {
			log.Println("[Status] No Reply inbox, skipping...")
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

// Shutdown - close nats connection
func (c *Component) Shutdown() error {
	c.NATS().Close()
	return nil
}
