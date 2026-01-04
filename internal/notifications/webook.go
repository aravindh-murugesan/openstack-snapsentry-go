package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func (w *Webhook) Notify(notification SnapshotCreationFailure) error {

	payload, err := json.Marshal(notification)
	if err != nil {
		return err
	}

	client := http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest("POST", w.URL, bytes.NewBuffer(payload))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	if w.Username != "" || w.Password != "" {
		req.SetBasicAuth(w.Username, w.Password)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Failed to send notification via Webhook: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Failed to send notification via Webhook: %d", resp.StatusCode)
	}

	return nil
}
