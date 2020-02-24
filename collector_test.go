package main

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/go-openapi/strfmt"
	"github.com/prometheus/alertmanager/api/v2/models"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/mock"
)

type MockedAlertmanagerClient struct {
	mock.Mock
}

func (mock MockedAlertmanagerClient) ListSilences() (models.GettableSilences, error) {
	args := mock.Called()
	return args.Get(0).(models.GettableSilences), args.Error(1)
}

func TestNewAlertmanagerSilencesCollector_Conf_OK(t *testing.T) {
	conf, err := loadConfig("sample-config.yml")
	if err != nil {
		t.Error(err)
	}

	asc := NewAlertmanagerSilencesCollector(conf, &AlertmanagerClient{})
	got := asc.Config.AlertmanagerURL
	want := "http://localhost:9093/api/v2"

	if got != want {
		t.Errorf("got '%s' want '%s'", got, want)
	}

}

func TestDecorate_OK(t *testing.T) {
	id := "abcd-1234"
	comment := "test"
	createdBy := "developer"
	startsAt, _ := strfmt.ParseDateTime("2020-02-20T22:12:33.533Z")
	endsAt, _ := strfmt.ParseDateTime("2020-02-29T23:11:44.603Z")
	status := "active"
	name := "foo"
	value := "bar"
	isRegex := false

	gettable := &models.GettableSilence{
		ID:     &id,
		Status: &models.SilenceStatus{State: &status},
		Silence: models.Silence{
			Comment:   &comment,
			CreatedBy: &createdBy,
			EndsAt:    &endsAt,
			StartsAt:  &startsAt,
			Matchers: models.Matchers{
				&models.Matcher{Name: &name, Value: &value, IsRegex: &isRegex},
			},
		},
	}

	var want = struct {
		Status string
		Labels map[string]string
	}{
		"active", map[string]string{
			"id":        id,
			"comment":   comment,
			"createdBy": createdBy,
			"startsAt":  startsAt.String(),
			"endsAt":    endsAt.String(),
			"status":    status,
			name:        value,
		},
	}

	got := &Silence{Gettable: gettable}
	err := got.Decorate()
	if err != nil {
		t.Error(err)
	}

	if got.Status != want.Status {
		t.Errorf("got '%s' want '%s'", got.Status, want.Status)
	}

	if !reflect.DeepEqual(got.Labels, want.Labels) {
		t.Errorf("got '%v' want '%v'", got.Labels, want.Labels)
	}
}

func CallExporter(collector *AlertmanagerSilencesCollector) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "/metrics", nil)
	rr := httptest.NewRecorder()
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)
	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	handler.ServeHTTP(rr, req)
	return rr
}

func TestCollector_Collect_OK(t *testing.T) {
	mac := MockedAlertmanagerClient{}

	conf, err := loadConfig("sample-config.yml")
	if err != nil {
		t.Error(err)
	}

	id := "abcd-1234"
	state := "active"
	updatedAt, _ := strfmt.ParseDateTime("2020-02-21T13:12:21.232Z")
	comment := "Silence"
	createdBy := "developer"
	endsAt, _ := strfmt.ParseDateTime("2020-02-29T23:11:44.603Z")
	startsAt, _ := strfmt.ParseDateTime("2020-02-20T22:12:33.533Z")
	name := "foo"
	value := "bar"
	isRegex := false

	silenceList := models.GettableSilences{{
		ID:        &id,
		Status:    &models.SilenceStatus{State: &state},
		UpdatedAt: &updatedAt,
		Silence: models.Silence{
			Comment:   &comment,
			CreatedBy: &createdBy,
			EndsAt:    &endsAt,
			Matchers: models.Matchers{
				&models.Matcher{Name: &name, Value: &value, IsRegex: &isRegex},
			},
			StartsAt: &startsAt,
		},
	}}

	mac.On("ListSilences", mock.Anything).Return(silenceList, nil)

	asc := NewAlertmanagerSilencesCollector(conf, mac)

	rr := CallExporter(asc)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Wrong status code: got %v, want %v", status, http.StatusOK)
	}

	want := `# HELP alertmanager_silence Alertmanager silence extract
# TYPE alertmanager_silence gauge
alertmanager_silence{comment="Silence",createdBy="developer",endsAt="2020-02-29T23:11:44.603Z",foo="bar",id="abcd-1234",startsAt="2020-02-20T22:12:33.533Z",status="active"} 1
`

	if rr.Body.String() != want {
		t.Errorf("Unexpected body: got %v, want %v", rr.Body.String(), want)
	}
}
