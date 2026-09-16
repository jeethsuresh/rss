import SwiftUI

private enum ReadingMode: String, CaseIterable, Identifiable {
    case rss
    case crawled
    case original

    var id: Self { self }

    var title: String {
        switch self {
        case .rss: "RSS"
        case .crawled: "Crawled"
        case .original: "Original"
        }
    }

    var symbol: String {
        switch self {
        case .rss: "dot.radiowaves.left.and.right"
        case .crawled: "text.page.fill"
        case .original: "globe"
        }
    }

    var explanation: String {
        switch self {
        case .rss: "The article exactly as supplied by its feed"
        case .crawled: "The clean reading copy prepared by your server"
        case .original: "A fresh server snapshot of the original page"
        }
    }
}

struct ReaderView: View {
    @Environment(SessionStore.self) private var session
    @Environment(\.openURL) private var openURL
    @State private var article: Article
    @State private var mode: ReadingMode
    @State private var busy = false
    @State private var modeBusy = false
    @Namespace private var modeIndicator
    private let originalOnly: Bool
    private let onChange: ((Article) -> Void)?

    init(article: Article, originalOnly: Bool = false, onChange: ((Article) -> Void)? = nil) {
        _article = State(initialValue: article)
        let hasCrawled = !(article.readerContent ?? "").trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
            || !article.crawledContent.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
        _mode = State(initialValue: originalOnly ? .original : (hasCrawled ? .crawled : .rss))
        self.originalOnly = originalOnly
        self.onChange = onChange
    }

    var body: some View {
        VStack(spacing: 0) {
            CompactArticleHeader(article: article, showsReadLater: originalOnly)
            Divider()

            Group {
                if modeBusy {
                    VStack(spacing: 14) {
                        ProgressView().controlSize(.large).tint(JoyPalette.violet)
                        Text("Loading the original from your server…")
                            .font(.callout.weight(.semibold))
                            .foregroundStyle(.secondary)
                    }
                    .frame(maxWidth: .infinity, maxHeight: .infinity)
                } else if let html = selectedHTML {
                    ReaderHTMLView(html: html, baseURL: URL(string: article.url))
                        .accessibilityLabel("\(mode.title) article content")
                } else {
                    unavailableMode
                }
            }
            .id(mode)
            .transition(.asymmetric(
                insertion: .move(edge: .trailing).combined(with: .opacity),
                removal: .move(edge: .leading).combined(with: .opacity)
            ))
        }
        .background(Color(.systemBackground))
        .safeAreaInset(edge: .top, spacing: 0) {
            if !originalOnly {
                ReadingModePicker(selection: $mode, namespace: modeIndicator)
                    .padding(.horizontal, 12)
                    .padding(.vertical, 9)
                    .background(.ultraThinMaterial)
                    .overlay(alignment: .bottom) { Divider().opacity(0.5) }
            }
        }
        .navigationBarTitleDisplayMode(.inline)
        .toolbar { readerToolbar }
        .task {
            await markReadOnOpenIfNeeded()
            if originalOnly && article.liveContent.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
                await fetchOriginal()
            }
        }
        .onChange(of: mode) { _, newMode in
            guard newMode == .original, article.liveContent.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else { return }
            Task { await fetchOriginal() }
        }
        .disabled(busy)
        .sensoryFeedback(.selection, trigger: mode)
        .sensoryFeedback(.selection, trigger: article.isRead)
        .sensoryFeedback(.success, trigger: article.isStarred)
    }

    @ToolbarContentBuilder
    private var readerToolbar: some ToolbarContent {
        ToolbarItemGroup(placement: .bottomBar) {
            Button(article.isRead ? "Unread" : "Read", systemImage: article.isRead ? "circle" : "checkmark.circle.fill") {
                Task { await setRead(!article.isRead) }
            }
            Spacer()
            Button(article.isStarred ? "Unstar" : "Star", systemImage: article.isStarred ? "star.fill" : "star") {
                Task { await mutate { try await session.api.toggleArticleStar(id: article.id) } }
            }
            .tint(article.isStarred ? JoyPalette.sunflower : JoyPalette.violet)
            Spacer()
            if !originalOnly && !article.isReadLater {
                Button("Read Later", systemImage: "bookmark.fill") {
                    Task {
                        do { _ = try await session.api.addReadLater(articleID: article.id) }
                        catch { session.report(error) }
                    }
                }
                Spacer()
            }
            if let url = URL(string: article.url) {
                ShareLink(item: url) { Label("Share", systemImage: "square.and.arrow.up") }.labelStyle(.iconOnly)
                Spacer()
                Button("Open website", systemImage: "safari") { openURL(url) }
            }
        }
        if !originalOnly {
            ToolbarItem(placement: .topBarTrailing) {
                Menu {
                    Button("Ask server to recrawl", systemImage: "arrow.clockwise") {
                        Task { await mutate { try await session.api.recrawlArticle(id: article.id) } }
                    }
                } label: { Image(systemName: "ellipsis.circle") }
            }
        }
    }

    private var selectedHTML: String? {
        let candidates: [String?]
        switch mode {
        case .rss:
            candidates = [article.rssContent, article.content, article.summary]
        case .crawled:
            candidates = [article.extractStatus == "ok" ? article.readerContent : nil, article.crawledContent, article.readerContent]
        case .original:
            candidates = [article.liveContent]
        }
        return candidates
            .compactMap { $0?.trimmingCharacters(in: .whitespacesAndNewlines) }
            .first { !$0.isEmpty }
    }

    private var unavailableMode: some View {
        VStack(spacing: 16) {
            Image(systemName: mode.symbol)
                .font(.system(size: 44, weight: .bold))
                .foregroundStyle(JoyPalette.violet)
                .symbolEffect(.breathe)
            Text("No \(mode.title.lowercased()) copy yet")
                .font(.system(.title3, design: .rounded, weight: .bold))
            Text(mode == .crawled ? "Ask the server to crawl this article again." : "The server did not return content for this mode.")
                .multilineTextAlignment(.center)
                .foregroundStyle(.secondary)
            if mode == .crawled {
                Button("Retry crawl", systemImage: "arrow.clockwise") {
                    Task { await mutate { try await session.api.recrawlArticle(id: article.id) } }
                }
                .buttonStyle(.borderedProminent)
                .tint(JoyPalette.violet)
            } else if mode == .original {
                Button("Fetch original", systemImage: "globe") { Task { await fetchOriginal() } }
                    .buttonStyle(.borderedProminent)
                    .tint(JoyPalette.violet)
            }
        }
        .padding(26)
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }

    private func fetchOriginal() async {
        modeBusy = true
        defer { modeBusy = false }
        do {
            article = try await session.api.fetchLiveArticle(id: article.id)
            onChange?(article)
        } catch { session.report(error) }
    }

    private func markReadOnOpenIfNeeded() async {
        do {
            let settings = try await session.api.settings()
            if settings.markReadOnOpen && !article.isRead { await setRead(true) }
        } catch { session.report(error) }
    }

    private func setRead(_ read: Bool) async {
        await mutate { try await session.api.setArticleRead(id: article.id, read: read) }
    }

    private func mutate(_ operation: () async throws -> Article) async {
        busy = true
        defer { busy = false }
        do {
            article = try await operation()
            onChange?(article)
        } catch { session.report(error) }
    }
}

private struct ReadingModePicker: View {
    @Binding var selection: ReadingMode
    let namespace: Namespace.ID

    var body: some View {
        VStack(spacing: 7) {
            HStack(spacing: 5) {
                ForEach(ReadingMode.allCases) { mode in
                    Button {
                        withAnimation(.snappy(duration: 0.34, extraBounce: 0.08)) {
                            selection = mode
                        }
                    } label: {
                        Label(mode.title, systemImage: mode.symbol)
                            .font(.subheadline.weight(.bold))
                            .foregroundStyle(selection == mode ? .white : .primary)
                            .frame(maxWidth: .infinity)
                            .padding(.vertical, 10)
                            .background {
                                if selection == mode {
                                    Capsule()
                                        .fill(JoyPalette.primary)
                                        .matchedGeometryEffect(id: "reader-mode", in: namespace)
                                        .shadow(color: JoyPalette.violet.opacity(0.24), radius: 8, y: 4)
                                }
                            }
                    }
                    .buttonStyle(.plain)
                    .accessibilityAddTraits(selection == mode ? .isSelected : [])
                }
            }
            Text(selection.explanation)
                .font(.caption)
                .foregroundStyle(.secondary)
                .contentTransition(.opacity)
                .animation(.easeOut(duration: 0.2), value: selection)
        }
    }
}

private struct CompactArticleHeader: View {
    let article: Article
    let showsReadLater: Bool

    var body: some View {
        HStack(alignment: .top, spacing: 14) {
            VStack(alignment: .leading, spacing: 8) {
                HStack(spacing: 7) {
                    FeedIconView(url: nil, title: article.feedTitle ?? "Article", size: 24)
                    Text(showsReadLater ? "READ LATER · ORIGINAL" : (article.feedTitle?.uppercased() ?? "ARTICLE"))
                        .font(.caption2.weight(.black))
                        .tracking(0.8)
                        .foregroundStyle(JoyPalette.violet)
                }
                Text(article.title.isEmpty ? article.url : article.title)
                    .font(.system(.title2, design: .serif, weight: .bold))
                    .textSelection(.enabled)
                    .lineLimit(3)
                HStack(spacing: 8) {
                    if let author = article.author.nilIfBlank { Text(author) }
                    if let date = article.publishedAt?.serverDate { Text(date, style: .date) }
                }
                .font(.caption)
                .foregroundStyle(.secondary)
            }
            .frame(maxWidth: .infinity, alignment: .leading)

            if article.leadImageURL != nil {
                ArticleArtwork(article: article, width: 92, height: 92)
            }
        }
        .padding(14)
        .background(Color(.secondarySystemBackground))
    }
}

private extension String {
    var nilIfBlank: String? { trimmingCharacters(in: .whitespacesAndNewlines).isEmpty ? nil : self }
}
