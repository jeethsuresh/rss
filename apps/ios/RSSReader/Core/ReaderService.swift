import Foundation

private struct IDParams: Codable, Sendable { let id: String }
private struct FeedURLParams: Codable, Sendable { let url: String }
private struct FeedEnabledParams: Codable, Sendable { let id: String; let enabled: Bool }
private struct FeedPollParams: Codable, Sendable { let id: String; let seconds: Int }
private struct ImportParams: Codable, Sendable { let text: String }
private struct ReadLaterListParams: Codable, Sendable { let filter: String?; let search: String? }
private struct ArticleIDParams: Codable, Sendable { let articleId: String }
private struct FollowedParams: Codable, Sendable { let teamIds: [Int] }
private struct TeamParams: Codable, Sendable { let teamId: Int; let season: Int? }
private struct SeasonParams: Codable, Sendable { let season: Int? }
private struct DateParams: Codable, Sendable { let date: String }
private struct GameParams: Codable, Sendable { let gamePk: Int }
private struct YearParams: Codable, Sendable { let year: Int? }
private struct SessionParams: Codable, Sendable { let sessionKey: Int }
private struct VoteParams: Codable, Sendable { let id: String; let vote: String }
private struct ArticleVoteParams: Codable, Sendable { let storyId: String; let articleId: String; let vote: String }
private struct FolderNameParams: Codable, Sendable { let name: String }
private struct FolderFeedParams: Codable, Sendable { let folderId: String; let feedId: String }
private struct ScanParams: Codable, Sendable { let window: String }
private struct LimitParams: Codable, Sendable { let limit: Int }

extension APIClient {
    func listFeeds() async throws -> [Feed] { try await rpc("feeds.list") }
    func feed(id: String) async throws -> Feed { try await rpc("feeds.get", params: IDParams(id: id)) }
    func previewFeed(url: String) async throws -> FeedPreview { try await rpc("feeds.preview", params: FeedURLParams(url: url)) }
    func addFeed(url: String) async throws -> Feed { try await rpc("feeds.add", params: FeedURLParams(url: url)) }
    func removeFeed(id: String) async throws { let _: EmptyResponse = try await rpc("feeds.remove", params: IDParams(id: id)) }
    func refreshFeed(id: String) async throws { let _: EmptyResponse = try await rpc("feeds.refresh", params: IDParams(id: id)) }
    func refreshAllFeeds() async throws { let _: EmptyResponse = try await rpc("feeds.refreshAll") }
    func setFeedEnabled(id: String, enabled: Bool) async throws -> Feed {
        try await rpc("feeds.setEnabled", params: FeedEnabledParams(id: id, enabled: enabled))
    }
    func setFeedPollInterval(id: String, seconds: Int) async throws -> Feed {
        try await rpc("feeds.setPollInterval", params: FeedPollParams(id: id, seconds: seconds))
    }
    func exportFeedURLs() async throws -> String { try await rpc("feeds.exportUrls", as: TextResult.self).text }
    func importFeedURLs(_ text: String) async throws -> FeedImportResult {
        try await rpc("feeds.importUrls", params: ImportParams(text: text))
    }

    func listArticles(_ query: ArticleQuery) async throws -> ArticleListResult {
        try await rpc("articles.list", params: query)
    }
    func article(id: String) async throws -> Article { try await rpc("articles.get", params: IDParams(id: id)) }
    func setArticleRead(id: String, read: Bool) async throws -> Article {
        try await rpc(read ? "articles.markRead" : "articles.markUnread", params: IDParams(id: id))
    }
    func toggleArticleStar(id: String) async throws -> Article { try await rpc("articles.toggleStar", params: IDParams(id: id)) }
    func markAllRead(_ query: ArticleQuery) async throws -> Int {
        try await rpc("articles.markAllRead", params: query, as: MarkAllReadResult.self).updated
    }
    func recrawlArticle(id: String) async throws -> Article { try await rpc("articles.recrawl", params: IDParams(id: id)) }
    func fetchLiveArticle(id: String) async throws -> Article { try await rpc("articles.fetchLive", params: IDParams(id: id)) }

    func listReadLater(filter: String?, search: String?) async throws -> [Article] {
        try await rpc("readLater.list", params: ReadLaterListParams(filter: filter, search: search))
    }
    func addReadLater(url: String) async throws -> Article { try await rpc("readLater.add", params: FeedURLParams(url: url)) }
    func addReadLater(articleID: String) async throws -> Article {
        try await rpc("readLater.addFromArticle", params: ArticleIDParams(articleId: articleID))
    }
    func archiveReadLater(id: String, archived: Bool) async throws -> Article {
        try await rpc(archived ? "readLater.archive" : "readLater.unarchive", params: IDParams(id: id))
    }
    func removeReadLater(id: String) async throws { let _: EmptyResponse = try await rpc("readLater.remove", params: IDParams(id: id)) }

    func listStories() async throws -> [Story] { try await rpc("stories.list") }
    func story(id: String) async throws -> Story { try await rpc("stories.get", params: IDParams(id: id)) }
    func setStoryRead(id: String, read: Bool) async throws -> Story {
        try await rpc(read ? "stories.markRead" : "stories.markUnread", params: IDParams(id: id))
    }
    func toggleStoryStar(id: String) async throws -> Story { try await rpc("stories.toggleStar", params: IDParams(id: id)) }
    func voteStory(id: String, vote: StoryVote) async throws -> Story {
        try await rpc("stories.voteStory", params: VoteParams(id: id, vote: vote.rawValue))
    }
    func voteArticle(storyID: String, articleID: String, vote: StoryVote) async throws -> Story {
        try await rpc("stories.voteArticle", params: ArticleVoteParams(storyId: storyID, articleId: articleID, vote: vote.rawValue))
    }
    func reindexStories() async throws -> Int { try await rpc("stories.reindex", as: StoryCountResult.self).storyCount }
    func splitStory(id: String) async throws -> [String] {
        try await rpc("stories.split", params: IDParams(id: id), as: StoryIDsResult.self).storyIds
    }

    func listFolders() async throws -> [Folder] { try await rpc("folders.list") }
    func createFolder(name: String) async throws -> Folder { try await rpc("folders.create", params: FolderNameParams(name: name)) }
    func removeFolder(id: String) async throws { let _: EmptyResponse = try await rpc("folders.remove", params: IDParams(id: id)) }
    func assignFeed(folderID: String, feedID: String, assigned: Bool) async throws {
        let _: EmptyResponse = try await rpc(
            assigned ? "folders.assignFeed" : "folders.unassignFeed",
            params: FolderFeedParams(folderId: folderID, feedId: feedID)
        )
    }

    func settings() async throws -> ReaderSettings { try await rpc("settings.get") }
    func updateSettings(_ settings: ReaderSettings) async throws -> ReaderSettings {
        try await rpc("settings.update", params: settings)
    }

    func mlbTeams() async throws -> [MlbTeam] { try await rpc("sports.teams.list") }
    func mlbSeasons() async throws -> [MlbSeason] { try await rpc("sports.seasons.list") }
    func followedTeams() async throws -> [Int] { try await rpc("sports.followed.get") }
    func setFollowedTeams(_ ids: [Int]) async throws -> [Int] { try await rpc("sports.followed.set", params: FollowedParams(teamIds: ids)) }
    func mlbSchedule(teamID: Int, season: Int?) async throws -> [MlbGame] {
        try await rpc("sports.schedule.list", params: TeamParams(teamId: teamID, season: season))
    }
    func mlbDaily(date: String) async throws -> [MlbGame] { try await rpc("sports.schedule.daily", params: DateParams(date: date)) }
    func mlbGame(id: Int) async throws -> MlbGameDetail { try await rpc("sports.game.get", params: GameParams(gamePk: id)) }
    func watchMlbGame(id: Int) async throws -> MlbGameDetail { try await rpc("sports.game.watch", params: GameParams(gamePk: id)) }
    func unwatchMlbGame(id: Int) async throws { let _: OkayResponse = try await rpc("sports.game.unwatch", params: GameParams(gamePk: id)) }
    func mlbStandings(season: Int?) async throws -> MlbStandings { try await rpc("sports.standings.get", params: SeasonParams(season: season)) }
    func mlbRoster(teamID: Int, season: Int?) async throws -> MlbRoster {
        try await rpc("sports.roster.get", params: TeamParams(teamId: teamID, season: season))
    }
    func f1Years() async throws -> [F1Season] { try await rpc("sports.f1.years.list") }
    func f1Races(year: Int?) async throws -> [F1Race] { try await rpc("sports.f1.races.list", params: YearParams(year: year)) }
    func f1Race(id: Int) async throws -> F1RaceDetail { try await rpc("sports.f1.race.get", params: SessionParams(sessionKey: id)) }
    func watchF1Race(id: Int) async throws -> F1RaceDetail { try await rpc("sports.f1.race.watch", params: SessionParams(sessionKey: id)) }
    func unwatchF1Race(id: Int) async throws { let _: OkayResponse = try await rpc("sports.f1.race.unwatch", params: SessionParams(sessionKey: id)) }
    func f1Standings(year: Int?) async throws -> F1Standings { try await rpc("sports.f1.standings.get", params: YearParams(year: year)) }

    func systemInfo() async throws -> SystemInfo { try await rpc("system.info") }
    func aiStatus() async throws -> AIStatus { try await rpc("ai.status") }
    func testAI() async throws -> AITestResult { try await rpc("ai.test") }
    func scanAI(window: String) async throws -> AIScanResult { try await rpc("ai.scan", params: ScanParams(window: window)) }
    func retryAI() async throws -> AIRetryResult { try await rpc("ai.retryFailed") }
    func aiLogs(limit: Int = 100) async throws -> [AILogEntry] { try await rpc("ai.logs", params: LimitParams(limit: limit)) }
}
