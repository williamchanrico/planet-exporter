package prometheus

import (
	"crypto/tls"
	"net/http"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/prom2json"
)

func Scrape(url string) ([]*prom2json.Family, error) {
	var err error

	mfChan := make(chan *dto.MetricFamily, 1024)

	transport, err := makeTransport()
	if err != nil {
		return nil, err
	}
	err = prom2json.FetchMetricFamilies(url, mfChan, transport)
	if err != nil {
		return nil, err
	}

	result := []*prom2json.Family{}
	for mf := range mfChan {
		result = append(result, prom2json.NewFamily(mf))
	}

	return result, nil
}

func makeTransport() (*http.Transport, error) {
	var transport *http.Transport
	transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return transport, nil
}
