package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	log "github.com/sirupsen/logrus"
)

// Host contains an inventory entry.
type Host struct {
	Domain    string `json:"domain"`
	Hostgroup string `json:"hostgroup"`
	IPAddress string `json:"ip_address"`
}

// requestHosts requests a new inventory host entries from upstream inventoryAddr.
func requestHosts(ctx context.Context, httpClient *http.Client, inventoryFormat, inventoryAddr string) ([]Host, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, inventoryAddr, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating inventory request: %w", err)
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("error requesting inventory: %w", err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			log.Errorf("error closing hosts response body: %v", err)
		}
	}()

	return parseHosts(inventoryFormat, response.Body)
}

// parseHosts parses inventory data as a list of Host.
func parseHosts(format string, data io.Reader) ([]Host, error) {
	var result []Host

	decoder := json.NewDecoder(data)
	decoder.DisallowUnknownFields()

	switch format {
	case fmtNDJSON:
		var inventoryEntry Host
		for decoder.More() {
			err := decoder.Decode(&inventoryEntry)
			if err != nil {
				log.Errorf("Skip an inventory host entry due to parser error: %v", err)

				continue
			}
			result = append(result, inventoryEntry)
		}

	case fmtArrayJSON:
		err := decoder.Decode(&result)
		if err != nil {
			return nil, fmt.Errorf("error decoding arrayjson inventory data: %w", err)
		}

		// Because we only expect a single JSON array object, we discard unexpected additional data.
		if decoder.More() {
			bytesCopied, _ := io.Copy(ioutil.Discard, data)
			log.Warnf("Unexpected remaining data (%v Bytes) while parsing inventory hosts", bytesCopied)
		}

	default:
		return nil, ErrInvalidInventoryFormat
	}
	log.Debugf("Parsed %v inventory hosts", len(result))

	return result, nil
}
