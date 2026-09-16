import Foundation
import Testing
@testable import RSSReader

struct CoreContractTests {
    @Test("Server URLs gain HTTPS and preserve an explicit development scheme")
    func serverURLNormalization() throws {
        #expect(try APIClient.normalizedServerURL("reader.example.com").absoluteString == "https://reader.example.com")
        #expect(try APIClient.normalizedServerURL("http://127.0.0.1:8787").absoluteString == "http://127.0.0.1:8787")
    }

    @Test("Tenant-safe SSE invalidations ignore forward-compatible payload fields")
    func eventDecoding() throws {
        let data = Data(#"{"event":"articles.added","payload":{"future":true}}"#.utf8)
        let event = try JSONDecoder().decode(BackendEvent.self, from: data)
        #expect(event.event == "articles.added")
    }

    @Test("RPC void responses accept the server's JSON null")
    func emptyResponseDecoding() throws {
        _ = try JSONDecoder().decode(EmptyResponse.self, from: Data("null".utf8))
    }

    @Test("Settings round trip through the server contract")
    func settingsRoundTrip() throws {
        let settings = ReaderSettings(
            defaultPollIntervalSeconds: 900,
            theme: "system",
            articleDensity: "comfortable",
            defaultSort: "newest",
            markReadOnOpen: true,
            notificationsEnabled: false,
            aiEnabled: true,
            aiBaseUrl: "https://ai.example.com/v1",
            aiModel: "reader-model",
            readLaterChrome: "tabs"
        )
        let decoded = try JSONDecoder().decode(ReaderSettings.self, from: JSONEncoder().encode(settings))
        #expect(decoded == settings)
    }

    @Test("Article cards resolve server-delivered lead images")
    func articleLeadImage() {
        let html = #"<article><img src="/media/cover.jpg?size=large&amp;fit=crop"></article>"#

        let image = Article.leadImageURL(in: html, relativeTo: "https://news.example.com/story/42")

        #expect(image?.absoluteString == "https://news.example.com/media/cover.jpg?size=large&fit=crop")
        #expect(Article.leadImageURL(in: #"<img src="data:image/png;base64,abc">"#, relativeTo: "https://news.example.com") == nil)
    }

    @Test("OpenF1 country codes map to real flags")
    @MainActor
    func formulaOneFlags() {
        let britishFlag = CountryFlagView.flag(code: "GBR", countryName: "United Kingdom")
        let monacoFlag = CountryFlagView.flag(code: "MON", countryName: "Monaco")
        let emiratesFlag = CountryFlagView.flag(code: "UAE", countryName: "United Arab Emirates")
        let canadianFlag = CountryFlagView.flag(code: nil, countryName: "Canada")

        #expect(britishFlag == "🇬🇧")
        #expect(monacoFlag == "🇲🇨")
        #expect(emiratesFlag == "🇦🇪")
        #expect(canadianFlag == "🇨🇦")
    }

    @Test("Sports dashboards prioritize results and adjacent races")
    func sportsPriority() throws {
        let games = try JSONDecoder().decode([MlbGame].self, from: Data(#"""
        [
          {"id":1,"season":2026,"gameDate":"2026-08-27T19:00:00Z","status":"final","awayTeam":{"id":1,"name":"A","abbreviation":"A"},"homeTeam":{"id":2,"name":"B","abbreviation":"B"},"awayScore":4,"homeScore":2},
          {"id":2,"season":2026,"gameDate":"2026-08-28T19:00:00Z","status":"scheduled","awayTeam":{"id":3,"name":"C","abbreviation":"C"},"homeTeam":{"id":4,"name":"D","abbreviation":"D"}},
          {"id":3,"season":2026,"gameDate":"2026-08-28T18:00:00Z","status":"final","awayTeam":{"id":5,"name":"E","abbreviation":"E"},"homeTeam":{"id":6,"name":"F","abbreviation":"F"},"awayScore":1,"homeScore":2}
        ]
        """#.utf8))
        #expect(SportsPresentation.latestGames(from: games).map(\.id) == [3, 1])
        #expect(SportsPresentation.latestGames(from: games, trackedTeamIDs: [1]).map(\.id) == [1, 3])

        let races = try JSONDecoder().decode([F1Race].self, from: Data(#"""
        [
          {"meetingKey":1,"sessionKey":11,"year":2026,"name":"Previous","location":"A","countryName":"Canada","circuitShortName":"A","dateStart":"2026-08-20T18:00:00Z","dateEnd":"2026-08-20T20:00:00Z","status":"completed"},
          {"meetingKey":2,"sessionKey":22,"year":2026,"name":"Next","location":"B","countryName":"Italy","circuitShortName":"B","dateStart":"2026-09-05T18:00:00Z","dateEnd":"2026-09-05T20:00:00Z","status":"scheduled"}
        ]
        """#.utf8))
        let now = try #require("2026-08-28T12:00:00Z".serverDate)
        #expect(SportsPresentation.previousRace(in: races, now: now)?.name == "Previous")
        #expect(SportsPresentation.nextRace(in: races, now: now)?.name == "Next")
    }

    @Test("Detailed MLB statistics decode from the server contract")
    func detailedMLBStatistics() throws {
        let batting = try JSONDecoder().decode(MlbBattingStats.self, from: Data(#"""
        {"games":120,"plateAppearances":510,"atBats":455,"runs":82,"hits":137,"doubles":29,"triples":4,"homeRuns":31,"rbi":91,"walks":48,"strikeOuts":96,"stolenBases":18,"average":".301","onBasePercentage":".372","slugging":".587","ops":".959"}
        """#.utf8))
        #expect(batting.plateAppearances == 510)
        #expect(batting.stolenBases == 18)
        #expect(batting.ops == ".959")

        let box = try JSONDecoder().decode(MlbTeamBox.self, from: Data(#"""
        {
          "team":{"id":1,"name":"Tracked Club","abbreviation":"TCL"},
          "batters":[{"playerId":10,"name":"Slugger","position":"RF","battingOrder":3,"atBats":4,"runs":2,"hits":3,"rbi":4,"walks":1,"strikeOuts":0,"homeRuns":1}],
          "pitchers":[{"playerId":20,"name":"Starter","inningsPitched":"7.0","hits":4,"runs":1,"earnedRuns":1,"walks":2,"strikeOuts":9,"homeRuns":0,"pitchesThrown":101,"strikes":69}]
        }
        """#.utf8))
        #expect(box.batters.first?.homeRuns == 1)
        #expect(box.pitchers.first?.strikeOuts == 9)
        #expect(box.pitchers.first?.strikes == 69)
    }
}

@MainActor
struct SessionStoreErrorReportingTests {
    @Test("Swift task cancellation is not presented as an error")
    func ignoresTaskCancellation() {
        let session = SessionStore()

        session.report(CancellationError())

        #expect(session.alertMessage == nil)
    }

    @Test("URLSession cancellation is not presented as an error")
    func ignoresURLSessionCancellation() {
        let session = SessionStore()

        session.report(URLError(.cancelled))

        #expect(session.alertMessage == nil)
    }

    @Test("Actionable failures are still presented")
    func reportsActionableErrors() {
        let session = SessionStore()

        session.report(APIClientError.invalidResponse)

        #expect(session.alertMessage == "The server returned an unreadable response.")
    }
}
