import Foundation

struct User: Codable, Sendable, Hashable {
    let id: String
    let username: String
    let createdAt: String?
}

struct AuthResult: Codable, Sendable {
    let token: String
    let expiresAt: String
    let user: User
}

struct ServerConfiguration: Codable, Sendable {
    let registrationEnabled: Bool
    let version: String
}

struct SystemInfo: Codable, Sendable {
    let version: String
    let dbPath: String
    let protocolVersion: Int
}

struct Feed: Codable, Identifiable, Sendable, Hashable {
    let id: String
    let url: String
    let title: String
    let description: String
    let siteUrl: String
    let iconUrl: String
    let lastSuccessAt: String?
    let lastAttemptAt: String?
    let lastError: String
    let pollIntervalSeconds: Int
    let enabled: Bool
    let unreadCount: Int
    let isReadLater: Bool
    let crawlAttempts: Int
    let crawlFailures: Int
    let badCrawlPercent: Double
    let createdAt: String
    let updatedAt: String
}

enum Priority: String, Codable, Sendable {
    case none, low, medium, high
}

struct Article: Codable, Identifiable, Sendable, Hashable {
    let id: String
    let feedId: String
    let title: String
    let url: String
    let author: String
    let content: String
    let summary: String
    let rssContent: String
    let crawledContent: String
    let liveContent: String
    let crawlStatus: String
    let crawlError: String
    let crawlUnreliable: Bool
    let crawlRetryable: Bool?
    let readerContent: String?
    let extractStatus: String?
    let extractSource: String?
    let publishedAt: String?
    let updatedAt: String?
    let externalId: String
    let isRead: Bool
    let isStarred: Bool
    let isReadLater: Bool
    let archivedAt: String?
    let priority: Priority
    let storyId: String?
    let discoveredAt: String
    let feedTitle: String?

    var displayHTML: String {
        [readerContent, crawledContent, content, rssContent, summary]
            .compactMap { $0 }
            .first { !$0.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty } ?? ""
    }

    var leadImageURL: URL? {
        Self.leadImageURL(in: displayHTML, relativeTo: url)
    }

    static func leadImageURL(in html: String, relativeTo articleURLString: String) -> URL? {
        guard !html.isEmpty,
              let expression = try? NSRegularExpression(
                  pattern: #"<img\b[^>]*?\bsrc\s*=\s*[\"']([^\"']+)[\"']"#,
                  options: [.caseInsensitive]
              ),
              let match = expression.firstMatch(in: html, range: NSRange(html.startIndex..., in: html)),
              let range = Range(match.range(at: 1), in: html) else {
            return nil
        }

        let source = String(html[range]).replacingOccurrences(of: "&amp;", with: "&")
        guard !source.hasPrefix("data:") else { return nil }
        if let absolute = URL(string: source), absolute.scheme != nil {
            return absolute
        }
        guard let articleURL = URL(string: articleURLString) else { return nil }
        return URL(string: source, relativeTo: articleURL)?.absoluteURL
    }
}

struct ArticleQuery: Codable, Sendable, Equatable {
    var feedId: String?
    var folderId: String?
    var unreadOnly: Bool?
    var starredOnly: Bool?
    var search: String?
    var limit: Int? = 50
    var cursor: String?
}

struct ArticleListResult: Codable, Sendable {
    let articles: [Article]
    let nextCursor: String?
}

enum StoryVote: String, Codable, Sendable {
    case none = ""
    case up
    case down
}

struct Story: Codable, Identifiable, Sendable, Hashable {
    let id: String
    let title: String
    let summary: String
    let source: String?
    let vote: StoryVote?
    let articleVotes: [String: StoryVote]?
    let isRead: Bool
    let isStarred: Bool
    let memberCount: Int
    let createdAt: String
    let updatedAt: String
    let articleIds: [String]?
    let articles: [Article]?
}

struct Folder: Codable, Identifiable, Sendable, Hashable {
    let id: String
    let name: String
    let createdAt: String
    let feedIds: [String]
}

struct FeedPreview: Codable, Sendable {
    let url: String
    let title: String
    let description: String
    let siteUrl: String
    let articleCount: Int
}

struct ReaderSettings: Codable, Sendable, Equatable {
    var defaultPollIntervalSeconds: Int
    var theme: String
    var articleDensity: String
    var defaultSort: String
    var markReadOnOpen: Bool
    var notificationsEnabled: Bool
    var aiEnabled: Bool
    var aiBaseUrl: String
    var aiModel: String
    var readLaterChrome: String?
}

struct AIStatus: Codable, Sendable {
    let running: Bool
    let processed: Int
    let total: Int
    let pending: Int
    let failed: Int
    let lastError: String
}

struct AILogEntry: Codable, Identifiable, Sendable {
    let id: String
    let ts: String
    let level: String
    let articleId: String?
    let message: String
    let detail: String?
}

struct AITestResult: Codable, Sendable {
    let ok: Bool
    let message: String
    let models: [String]?
}

struct MlbTeam: Codable, Identifiable, Sendable, Hashable {
    let id: Int
    let name: String
    let abbreviation: String
    let shortName: String?
    let logoUrl: String?
    let league: String?
}

struct MlbSeason: Codable, Sendable, Hashable {
    let seasonId: Int
    let regularSeasonStartDate: String?
    let regularSeasonEndDate: String?
}

struct MlbGame: Codable, Identifiable, Sendable, Hashable {
    let id: Int
    let season: Int
    let gameDate: String
    let officialDate: String?
    let status: String
    let statusDetail: String?
    let awayTeam: MlbTeam
    let homeTeam: MlbTeam
    let awayScore: Int?
    let homeScore: Int?
    let currentInning: Int?
    let currentInningHalf: String?
    let league: String?
}

struct MlbStandingRow: Codable, Identifiable, Sendable, Hashable {
    var id: Int { team.id }
    let rank: Int
    let team: MlbTeam
    let wins: Int
    let losses: Int
    let winningPercentage: String
    let gamesBack: String
    let wildCardGamesBack: String?
    let runDifferential: Int
    let streak: String?
    let divisionLeader: Bool?
    let clinched: Bool?
}

struct MlbStandingSection: Codable, Identifiable, Sendable, Hashable {
    let id: String
    let league: String
    let name: String
    let kind: String
    let teams: [MlbStandingRow]
}

struct MlbStandings: Codable, Sendable {
    let season: Int
    let sections: [MlbStandingSection]
}

struct MlbPlay: Codable, Identifiable, Sendable, Hashable {
    let id: String
    let inning: Int
    let half: String
    let event: String
    let description: String
    let isScoringPlay: Bool
    let awayScore: Int?
    let homeScore: Int?
}

struct MlbInning: Codable, Identifiable, Sendable, Hashable {
    var id: Int { number }
    let number: Int
    let awayRuns: Int
    let homeRuns: Int
    let awayHits: Int?
    let homeHits: Int?
    let awayErrors: Int?
    let homeErrors: Int?
}

struct MlbGameDetail: Codable, Sendable {
    let game: MlbGame
    let innings: [MlbInning]
    let plays: [MlbPlay]
    let awayHits: Int?
    let homeHits: Int?
    let awayErrors: Int?
    let homeErrors: Int?
    let awayBox: MlbTeamBox?
    let homeBox: MlbTeamBox?
}

struct MlbBattingStats: Codable, Sendable, Hashable {
    let games: Int
    let plateAppearances: Int?
    let atBats: Int
    let runs: Int
    let hits: Int
    let doubles: Int?
    let triples: Int?
    let homeRuns: Int
    let rbi: Int
    let walks: Int
    let strikeOuts: Int
    let stolenBases: Int?
    let average: String
    let onBasePercentage: String
    let slugging: String
    let ops: String?
}

struct MlbPitchingStats: Codable, Sendable, Hashable {
    let games: Int
    let gamesStarted: Int
    let wins: Int
    let losses: Int
    let saves: Int
    let inningsPitched: String
    let era: String
    let whip: String
    let strikeOuts: Int
    let walks: Int
}

struct MlbRosterPlayer: Codable, Identifiable, Sendable, Hashable {
    var id: Int { playerId }
    let playerId: Int
    let name: String
    let jerseyNumber: String?
    let position: String?
    let positionType: String?
    let rosterStatus: String
    let statusDescription: String?
    let batting: MlbBattingStats?
    let pitching: MlbPitchingStats?
}

struct MlbRoster: Codable, Sendable {
    let teamId: Int
    let season: Int
    let players: [MlbRosterPlayer]
}

struct MlbBatterLine: Codable, Identifiable, Sendable, Hashable {
    var id: Int { playerId }
    let playerId: Int
    let name: String
    let position: String?
    let battingOrder: Int?
    let atBats: Int
    let runs: Int
    let hits: Int
    let rbi: Int
    let walks: Int
    let strikeOuts: Int
    let homeRuns: Int
    let summary: String?
}

struct MlbPitcherLine: Codable, Identifiable, Sendable, Hashable {
    var id: String { "\(playerId)-\(inningsPitched)-\(pitchesThrown)" }
    let playerId: Int
    let name: String
    let note: String?
    let inningsPitched: String
    let hits: Int
    let runs: Int
    let earnedRuns: Int
    let walks: Int
    let strikeOuts: Int
    let homeRuns: Int
    let pitchesThrown: Int
    let strikes: Int?
    let summary: String?
}

struct MlbTeamBox: Codable, Sendable, Hashable {
    let team: MlbTeam
    let batters: [MlbBatterLine]
    let pitchers: [MlbPitcherLine]
}

struct F1Season: Codable, Sendable, Hashable {
    let year: Int
}

struct F1Session: Codable, Identifiable, Sendable, Hashable {
    var id: Int { sessionKey }
    let sessionKey: Int
    let sessionName: String
    let sessionType: String?
    let kind: String
    let dateStart: String
    let dateEnd: String
    let status: String
}

struct F1Race: Codable, Identifiable, Sendable, Hashable {
    var id: Int { sessionKey }
    let meetingKey: Int
    let sessionKey: Int
    let year: Int
    let name: String
    let officialName: String?
    let location: String
    let countryName: String
    let countryCode: String?
    let circuitShortName: String
    let dateStart: String
    let dateEnd: String
    let status: String
    let sessions: [F1Session]?
}

struct F1DriverResult: Codable, Identifiable, Sendable, Hashable {
    var id: Int { driverNumber }
    let position: Int
    let driverNumber: Int
    let name: String
    let nameAcronym: String?
    let teamName: String?
    let points: Double
    let laps: Int
    let dnf: Bool
    let dns: Bool
    let dsq: Bool
    let gapToLeader: String?
}

struct F1Event: Codable, Identifiable, Sendable, Hashable {
    let id: String
    let date: String
    let category: String
    let flag: String?
    let lapNumber: Int?
    let driverNumber: Int?
    let driverName: String?
    let message: String
    let significant: Bool
}

struct F1RaceDetail: Codable, Sendable {
    let race: F1Race
    let session: F1Session?
    let results: [F1DriverResult]
    let events: [F1Event]
    let sessions: [F1Session]?
}

struct F1DriverStanding: Codable, Identifiable, Sendable, Hashable {
    var id: Int { driverNumber }
    let position: Int
    let driverNumber: Int
    let name: String
    let nameAcronym: String?
    let teamName: String?
    let points: Double
}

struct F1TeamStanding: Codable, Identifiable, Sendable, Hashable {
    var id: String { teamName }
    let position: Int
    let teamName: String
    let points: Double
}

struct F1Standings: Codable, Sendable {
    let year: Int
    let sessionKey: Int
    let meetingName: String?
    let drivers: [F1DriverStanding]
    let constructors: [F1TeamStanding]
}

struct BackendEvent: Codable, Sendable {
    let event: String
}

struct EmptyRequest: Encodable, Sendable {}
struct EmptyResponse: Decodable, Sendable {
    init() {}
    init(from decoder: Decoder) throws {
        let container = try decoder.singleValueContainer()
        if !container.decodeNil() {
            _ = try? container.decode([String: Bool].self)
        }
    }
}
struct OkayResponse: Codable, Sendable { let ok: Bool }
struct MarkAllReadResult: Codable, Sendable { let updated: Int }
struct StoryCountResult: Codable, Sendable { let storyCount: Int }
struct StoryIDsResult: Codable, Sendable { let storyIds: [String] }
struct TextResult: Codable, Sendable { let text: String }
struct FeedImportResult: Codable, Sendable { let added: Int; let failed: Int; let errors: [String] }
struct AIScanResult: Codable, Sendable { let queued: Bool; let status: AIStatus }
struct AIRetryResult: Codable, Sendable { let requeued: Int; let status: AIStatus }

extension String {
    var serverDate: Date? {
        let fractional = ISO8601DateFormatter()
        fractional.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        return fractional.date(from: self) ?? ISO8601DateFormatter().date(from: self)
    }
}
