package webhook

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/itallominatti/adquirente/internal/application/notification"
)

type HTTPSender struct{ client *http.Client }

var _ notification.Sender = (*HTTPSender)(nil)

func NewHTTPSender() *HTTPSender {
	return &HTTPSender{client: &http.Client{
		Timeout:       5 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirecionamento não permitido") },
	}}
}

func (s *HTTPSender) Send(ctx context.Context, url string, body []byte, headers map[string]string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("EC respondeu %d", resp.StatusCode)
	}
	return nil
}
