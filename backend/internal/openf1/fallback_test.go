package openf1_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jeeth/rss-reader/backend/internal/openf1"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

const liveSession401 = `{"detail":"Live F1 session in progress. Global API access (including past sessions) is restricted to authenticated users until the session ends."}`

const jolpicaRacesJSON = `{
  "MRData": {
    "RaceTable": {
      "season": "2024",
      "Races": [
        {
          "season": "2024",
          "round": "1",
          "raceName": "Bahrain Grand Prix",
          "Circuit": {
            "circuitName": "Bahrain International Circuit",
            "Location": {"locality": "Sakhir", "country": "Bahrain"}
          },
          "date": "2024-03-02",
          "time": "15:00:00Z",
          "FirstPractice": {"date": "2024-02-29", "time": "11:30:00Z"},
          "Qualifying": {"date": "2024-03-01", "time": "16:00:00Z"}
        }
      ]
    }
  }
}`

const jolpicaResultsJSON = `{
  "MRData": {
    "RaceTable": {
      "Races": [
        {
          "season": "2024",
          "round": "1",
          "raceName": "Bahrain Grand Prix",
          "Circuit": {
            "circuitName": "Bahrain International Circuit",
            "Location": {"locality": "Sakhir", "country": "Bahrain"}
          },
          "date": "2024-03-02",
          "time": "15:00:00Z",
          "Results": [
            {
              "number": "1",
              "position": "1",
              "positionText": "1",
              "points": "25",
              "laps": "57",
              "status": "Finished",
              "Driver": {
                "code": "VER",
                "givenName": "Max",
                "familyName": "Verstappen",
                "permanentNumber": "1"
              },
              "Constructor": {"name": "Red Bull"}
            }
          ]
        }
      ]
    }
  }
}`

const jolpicaDriverStandingsJSON = `{
  "MRData": {
    "StandingsTable": {
      "StandingsLists": [
        {
          "round": "1",
          "DriverStandings": [
            {
              "position": "1",
              "points": "25",
              "Driver": {
                "code": "VER",
                "givenName": "Max",
                "familyName": "Verstappen",
                "permanentNumber": "1"
              },
              "Constructors": [{"name": "Red Bull"}]
            }
          ]
        }
      ]
    }
  }
}`

const jolpicaConstructorStandingsJSON = `{
  "MRData": {
    "StandingsTable": {
      "StandingsLists": [
        {
          "round": "1",
          "ConstructorStandings": [
            {"position": "1", "points": "25", "Constructor": {"name": "Red Bull"}}
          ]
        }
      ]
    }
  }
}`

func TestListRacesFallsBackWhenOpenF1RestrictsLiveAccess(t *testing.T) {
	var openf1Hits, jolpicaHits int
	c := openf1.NewClient()
	c.HTTP = &http.Client{Timeout: 5 * time.Second, Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case strings.Contains(req.URL.Host, "openf1"):
			openf1Hits++
			if req.Header.Get("Authorization") != "" {
				t.Errorf("anonymous fallback request sent Authorization")
			}
			return jsonResponse(http.StatusUnauthorized, liveSession401), nil
		case strings.Contains(req.URL.Host, "jolpi"):
			jolpicaHits++
			if !strings.Contains(req.URL.Path, "/2024/") {
				t.Fatalf("unexpected jolpica path %s", req.URL.Path)
			}
			return jsonResponse(http.StatusOK, jolpicaRacesJSON), nil
		default:
			t.Fatalf("unexpected host %s", req.URL.Host)
			return jsonResponse(http.StatusNotFound, `{}`), nil
		}
	})}

	races, err := c.ListRaces(context.Background(), 2024)
	if err != nil {
		t.Fatalf("ListRaces: %v", err)
	}
	if jolpicaHits == 0 {
		t.Fatalf("expected Jolpica fallback after OpenF1 401, openf1Hits=%d", openf1Hits)
	}
	if len(races) != 1 || races[0].Name != "Bahrain Grand Prix" || races[0].Location != "Sakhir" {
		t.Fatalf("unexpected races: %+v", races)
	}
	if races[0].CountryCode != "BHR" {
		t.Fatalf("country code=%q", races[0].CountryCode)
	}
	if races[0].Status != "completed" {
		t.Fatalf("status=%s", races[0].Status)
	}
	if races[0].SessionKey == 0 || races[0].MeetingKey == 0 {
		t.Fatalf("missing keys: %+v", races[0])
	}
}

func TestRaceDetailFallsBackWhenOpenF1RestrictsLiveAccess(t *testing.T) {
	c := openf1.NewClient()
	c.HTTP = &http.Client{Timeout: 5 * time.Second, Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case strings.Contains(req.URL.Host, "openf1"):
			return jsonResponse(http.StatusUnauthorized, liveSession401), nil
		case strings.Contains(req.URL.Path, "/2024/races.json") || strings.HasSuffix(req.URL.Path, "/2024.json"):
			return jsonResponse(http.StatusOK, jolpicaRacesJSON), nil
		case strings.Contains(req.URL.Path, "/results.json"):
			return jsonResponse(http.StatusOK, jolpicaResultsJSON), nil
		default:
			t.Fatalf("unexpected request %s %s", req.URL.Host, req.URL.Path)
			return jsonResponse(http.StatusNotFound, `{}`), nil
		}
	})}

	races, err := c.ListRaces(context.Background(), 2024)
	if err != nil {
		t.Fatalf("ListRaces: %v", err)
	}
	detail, err := c.RaceDetail(context.Background(), races[0].SessionKey)
	if err != nil {
		t.Fatalf("RaceDetail: %v", err)
	}
	if detail.Race.Name != "Bahrain Grand Prix" {
		t.Fatalf("race=%+v", detail.Race)
	}
	if len(detail.Results) != 1 || detail.Results[0].NameAcronym != "VER" || detail.Results[0].Points != 25 {
		t.Fatalf("results=%+v", detail.Results)
	}
}

func TestStandingsFallBackWhenOpenF1RestrictsLiveAccess(t *testing.T) {
	c := openf1.NewClient()
	c.HTTP = &http.Client{Timeout: 5 * time.Second, Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case strings.Contains(req.URL.Host, "openf1"):
			return jsonResponse(http.StatusUnauthorized, liveSession401), nil
		case strings.Contains(req.URL.Path, "driverStandings"):
			return jsonResponse(http.StatusOK, jolpicaDriverStandingsJSON), nil
		case strings.Contains(req.URL.Path, "constructorStandings"):
			return jsonResponse(http.StatusOK, jolpicaConstructorStandingsJSON), nil
		case strings.Contains(req.URL.Path, "/2024/"):
			return jsonResponse(http.StatusOK, jolpicaRacesJSON), nil
		default:
			t.Fatalf("unexpected request %s %s", req.URL.Host, req.URL.Path)
			return jsonResponse(http.StatusNotFound, `{}`), nil
		}
	})}

	st, err := c.Standings(context.Background(), 2024)
	if err != nil {
		t.Fatalf("Standings: %v", err)
	}
	if len(st.Drivers) != 1 || st.Drivers[0].Name != "Max Verstappen" || st.Drivers[0].Points != 25 {
		t.Fatalf("drivers=%+v", st.Drivers)
	}
	if len(st.Constructors) != 1 || st.Constructors[0].TeamName != "Red Bull" {
		t.Fatalf("constructors=%+v", st.Constructors)
	}
}

func TestAuthenticatedOpenF1RequestsSendBearerToken(t *testing.T) {
	var sawBearer bool
	c := openf1.NewClient()
	c.Username = "user@example.com"
	c.Password = "secret"
	c.HTTP = &http.Client{Timeout: 5 * time.Second, Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/token" || strings.HasSuffix(req.URL.Path, "/token") {
			body, _ := io.ReadAll(req.Body)
			if !strings.Contains(string(body), "username=user%40example.com") || !strings.Contains(string(body), "password=secret") {
				t.Fatalf("token body=%s", body)
			}
			return jsonResponse(http.StatusOK, `{"access_token":"tok-123","token_type":"bearer","expires_in":"3600"}`), nil
		}
		if req.Header.Get("Authorization") != "Bearer tok-123" {
			t.Fatalf("missing bearer on %s", req.URL)
		}
		sawBearer = true
		if strings.Contains(req.URL.Path, "/meetings") {
			return jsonResponse(http.StatusOK, `[{"meeting_key":1,"meeting_name":"Bahrain Grand Prix","location":"Sakhir","country_name":"Bahrain","country_code":"BHR","circuit_short_name":"Bahrain","date_start":"2024-03-02T15:00:00+00:00","year":2024}]`), nil
		}
		if strings.Contains(req.URL.Path, "/sessions") {
			return jsonResponse(http.StatusOK, `[{"session_key":9472,"session_name":"Race","session_type":"Race","date_start":"2024-03-02T15:00:00+00:00","date_end":"2024-03-02T17:00:00+00:00","meeting_key":1,"circuit_short_name":"Bahrain","country_name":"Bahrain","country_code":"BHR","location":"Sakhir","year":2024}]`), nil
		}
		return jsonResponse(http.StatusOK, `[]`), nil
	})}

	races, err := c.ListRaces(context.Background(), 2024)
	if err != nil {
		t.Fatalf("ListRaces: %v", err)
	}
	if !sawBearer {
		t.Fatal("expected authenticated OpenF1 requests")
	}
	if len(races) != 1 || races[0].SessionKey != 9472 {
		t.Fatalf("expected OpenF1 race, got %+v", races)
	}
}

func TestLiveSession401IsNotRetried(t *testing.T) {
	hits := 0
	c := openf1.NewClient()
	c.HTTP = &http.Client{Timeout: 5 * time.Second, Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		hits++
		return jsonResponse(http.StatusUnauthorized, liveSession401), nil
	})}
	_, err := c.ListRaces(context.Background(), 2024)
	if err == nil {
		t.Fatal("expected error when fallback also fails")
	}
	if hits > 3 {
		encoded, _ := json.Marshal(hits)
		t.Fatalf("retried live-session 401 too many times: hits=%s err=%v", encoded, err)
	}
}
