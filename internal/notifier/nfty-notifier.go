// Package notifier mengurus pengiriman notifikasi push ke HP lewat ntfy.sh,
// serta penyimpanan pengaturan topiknya (config.json).
package notifier

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Notifier adalah kontrak pengiriman notifikasi, supaya layer handler
// tidak bergantung langsung pada implementasi ntfy.sh.
type Notifier interface {
	Send(title, message string)
	Topic() string
	SetTopic(topic string)
}

// NtfyNotifier mengirim notifikasi lewat layanan ntfy.sh.
type NtfyNotifier struct {
	mu         sync.Mutex
	configFile string
	topic      string
	client     *http.Client
}

func NewNtfyNotifier(configFile string) *NtfyNotifier {
	n := &NtfyNotifier{configFile: configFile, client: &http.Client{Timeout: 10 * time.Second}}
	n.load()
	return n
}

type config struct {
	NtfyTopic string `json:"ntfyTopic"`
}

func (n *NtfyNotifier) load() {
	raw, err := os.ReadFile(n.configFile)
	if err != nil {
		return
	}
	var c config
	if err := json.Unmarshal(raw, &c); err != nil {
		return
	}
	n.topic = c.NtfyTopic
}

func (n *NtfyNotifier) save() {
	raw, _ := json.MarshalIndent(config{NtfyTopic: n.topic}, "", "  ")
	_ = os.WriteFile(n.configFile, raw, 0644)
}

func (n *NtfyNotifier) Topic() string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.topic
}

func (n *NtfyNotifier) SetTopic(topic string) {
	n.mu.Lock()
	n.topic = strings.TrimSpace(topic)
	n.save()
	n.mu.Unlock()
}

func (n *NtfyNotifier) Send(title, message string) {
	topic := n.Topic()
	if topic == "" {
		return
	}
	req, err := http.NewRequest(http.MethodPost, "https://ntfy.sh/"+topic, bytes.NewBufferString(message))
	if err != nil {
		return
	}
	req.Header.Set("Title", title)
	resp, err := n.client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
}
