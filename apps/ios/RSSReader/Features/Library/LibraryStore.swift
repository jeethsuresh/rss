import Foundation
import Observation

@MainActor
@Observable
final class LibraryStore {
    enum Scope: Hashable {
        case unread, all, starred, stories, feed(String), folder(String)
    }

    var scope: Scope = .unread
    var search = ""
    private(set) var feeds: [Feed] = []
    private(set) var folders: [Folder] = []
    private(set) var articles: [Article] = []
    private(set) var stories: [Story] = []
    private(set) var nextCursor: String?
    private(set) var loading = false
    private(set) var loadingMore = false

    func load(using api: APIClient) async throws {
        loading = true
        defer { loading = false }
        async let feeds = api.listFeeds()
        async let folders = api.listFolders()
        switch scope {
        case .stories:
            let (newFeeds, newFolders, newStories) = try await (feeds, folders, api.listStories())
            self.feeds = newFeeds
            self.folders = newFolders
            stories = newStories.filter { $0.memberCount > 1 }
            articles = []
        default:
            let result = try await api.listArticles(query())
            let (newFeeds, newFolders) = try await (feeds, folders)
            self.feeds = newFeeds
            self.folders = newFolders
            articles = result.articles
            nextCursor = result.nextCursor
            stories = []
        }
    }

    func loadMore(using api: APIClient) async throws {
        guard !loadingMore, let nextCursor, scope != .stories else { return }
        loadingMore = true
        defer { loadingMore = false }
        var query = query()
        query.cursor = nextCursor
        let result = try await api.listArticles(query)
        articles.append(contentsOf: result.articles.filter { article in !articles.contains { $0.id == article.id } })
        self.nextCursor = result.nextCursor
    }

    func toggleRead(_ article: Article, using api: APIClient) async throws {
        replace(try await api.setArticleRead(id: article.id, read: !article.isRead))
    }

    func toggleStar(_ article: Article, using api: APIClient) async throws {
        replace(try await api.toggleArticleStar(id: article.id))
    }

    func save(_ article: Article, using api: APIClient) async throws {
        _ = try await api.addReadLater(articleID: article.id)
    }

    func markAllRead(using api: APIClient) async throws {
        _ = try await api.markAllRead(query())
        articles = articles.map { article in
            Article(
                id: article.id, feedId: article.feedId, title: article.title, url: article.url,
                author: article.author, content: article.content, summary: article.summary,
                rssContent: article.rssContent, crawledContent: article.crawledContent,
                liveContent: article.liveContent, crawlStatus: article.crawlStatus,
                crawlError: article.crawlError, crawlUnreliable: article.crawlUnreliable,
                crawlRetryable: article.crawlRetryable, readerContent: article.readerContent,
                extractStatus: article.extractStatus, extractSource: article.extractSource,
                publishedAt: article.publishedAt, updatedAt: article.updatedAt,
                externalId: article.externalId, isRead: true, isStarred: article.isStarred,
                isReadLater: article.isReadLater, archivedAt: article.archivedAt,
                priority: article.priority, storyId: article.storyId,
                discoveredAt: article.discoveredAt, feedTitle: article.feedTitle
            )
        }
    }

    func replace(_ article: Article) {
        if let index = articles.firstIndex(where: { $0.id == article.id }) { articles[index] = article }
    }

    private func query() -> ArticleQuery {
        var query = ArticleQuery()
        query.search = search.trimmingCharacters(in: .whitespacesAndNewlines).nilIfEmpty
        switch scope {
        case .unread: query.unreadOnly = true
        case .starred: query.starredOnly = true
        case let .feed(id): query.feedId = id
        case let .folder(id): query.folderId = id
        case .all, .stories: break
        }
        return query
    }
}

private extension String {
    var nilIfEmpty: String? { isEmpty ? nil : self }
}
