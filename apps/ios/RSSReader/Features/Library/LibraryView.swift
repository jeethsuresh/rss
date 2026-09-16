import SwiftUI

struct LibraryView: View {
    @Environment(SessionStore.self) private var session
    @State private var store = LibraryStore()
    @State private var showingAddFeed = false
    let scope: LibraryStore.Scope

    private struct LoadKey: Hashable {
        let scope: LibraryStore.Scope
        let search: String
        let generation: Int
    }

    init(scope: LibraryStore.Scope = .unread) {
        self.scope = scope
    }

    var body: some View {
        NavigationStack {
            Group {
                if store.loading && store.articles.isEmpty && store.stories.isEmpty {
                    LoadingOverlay(title: "Loading your library…")
                } else if scope == .stories {
                    storyList
                } else {
                    articleList
                }
            }
            .navigationTitle(scopeTitle)
            .navigationBarTitleDisplayMode(.inline)
            .searchable(text: $store.search, prompt: "Search articles")
            .toolbar {
                ToolbarItemGroup(placement: .topBarTrailing) {
                    if scope != .stories {
                        Button("Mark all read", systemImage: "checkmark.circle") {
                            Task { await markAllRead() }
                        }
                    }
                    Button("Add feed", systemImage: "plus") { showingAddFeed = true }
                }
            }
            .refreshable { await load() }
            .navigationDestination(for: Article.self) { article in
                ReaderView(article: article) { store.replace($0) }
            }
            .navigationDestination(for: Story.self) { story in
                StoryDetailView(story: story)
            }
            .sheet(isPresented: $showingAddFeed) { AddFeedView { await load() } }
            .task(id: LoadKey(scope: scope, search: store.search, generation: session.refreshGeneration)) {
                if !store.search.isEmpty { try? await Task.sleep(for: .milliseconds(300)) }
                guard !Task.isCancelled else { return }
                store.scope = scope
                await load()
            }
        }
    }

    private var articleList: some View {
        List {
            if store.articles.isEmpty && !store.loading {
                ContentUnavailableView("No articles", systemImage: "newspaper", description: Text("Pull to refresh or choose another source."))
                    .joyfulListRow()
            }
            ForEach(store.articles) { article in
                NavigationLink(value: article) {
                    ArticleRow(article: article, feedIconURL: feedIconURL(for: article))
                }
                    .buttonStyle(JoyPressStyle())
                    .swipeActions(edge: .leading, allowsFullSwipe: true) {
                        Button(article.isRead ? "Unread" : "Read", systemImage: article.isRead ? "circle" : "checkmark") {
                            Task { await mutate { try await store.toggleRead(article, using: session.api) } }
                        }
                        .tint(.blue)
                    }
                    .swipeActions(edge: .trailing, allowsFullSwipe: false) {
                        Button("Save", systemImage: "bookmark") {
                            Task { await mutate { try await store.save(article, using: session.api) } }
                        }
                        .tint(.indigo)
                        Button(article.isStarred ? "Unstar" : "Star", systemImage: article.isStarred ? "star.slash" : "star") {
                            Task { await mutate { try await store.toggleStar(article, using: session.api) } }
                        }
                        .tint(.yellow)
                    }
                    .onAppear {
                        if article.id == store.articles.last?.id {
                            Task { await loadMore() }
                        }
                    }
                    .joyfulListRow()
            }
            if store.loadingMore {
                HStack { Spacer(); ProgressView().tint(JoyPalette.violet); Spacer() }
                    .joyfulListRow()
            }
        }
        .listStyle(.plain)
        .scrollContentBackground(.hidden)
        .background { JoyfulBackdrop() }
    }

    private var storyList: some View {
        List {
            if store.stories.isEmpty && !store.loading {
                ContentUnavailableView(
                    "No meta-stories yet",
                    systemImage: "square.stack.3d.up",
                    description: Text("Your server groups related reporting here when multiple sources cover the same story.")
                )
                .joyfulListRow()
            }
            ForEach(store.stories) { story in
                NavigationLink(value: story) {
                    StoryCard(story: story)
                }
                .buttonStyle(JoyPressStyle())
                .joyfulListRow()
            }
        }
        .listStyle(.plain)
        .scrollContentBackground(.hidden)
        .background { JoyfulBackdrop() }
    }

    private var scopeTitle: String {
        switch scope {
        case .unread: "Unread"
        case .all: "All Articles"
        case .starred: "Starred"
        case .stories: "Meta-stories"
        case let .feed(id): store.feeds.first { $0.id == id }?.title ?? "Feed"
        case let .folder(id): store.folders.first { $0.id == id }?.name ?? "Folder"
        }
    }

    private func feedIconURL(for article: Article) -> URL? {
        guard let feed = store.feeds.first(where: { $0.id == article.feedId }) else { return nil }
        return URL(string: feed.iconUrl)
    }

    private func load() async {
        await mutate { try await store.load(using: session.api) }
    }

    private func loadMore() async {
        await mutate { try await store.loadMore(using: session.api) }
    }

    private func markAllRead() async {
        await mutate { try await store.markAllRead(using: session.api) }
    }

    private func mutate(_ operation: () async throws -> Void) async {
        do { try await operation() } catch { session.report(error) }
    }
}

private struct StoryCard: View {
    let story: Story

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack {
                StatusPill(text: "\(story.memberCount) sources", color: JoyPalette.cyan)
                Spacer()
                if story.isStarred {
                    Image(systemName: "star.fill")
                        .foregroundStyle(JoyPalette.sunflower)
                        .symbolEffect(.bounce, value: story.isStarred)
                }
            }
            HStack(alignment: .firstTextBaseline, spacing: 8) {
                if !story.isRead { Circle().fill(JoyPalette.coral).frame(width: 8, height: 8) }
                Text(story.title)
                    .font(.system(.title3, design: .rounded, weight: .bold))
                    .lineLimit(3)
            }
            Text(story.summary)
                .font(.subheadline)
                .foregroundStyle(.secondary)
                .lineLimit(3)
            HStack {
                Image(systemName: "sparkles")
                Text("Meta-story")
                Spacer()
                Image(systemName: "arrow.up.right")
            }
            .font(.caption.weight(.bold))
            .foregroundStyle(JoyPalette.violet)
        }
        .padding(18)
        .background(.background.opacity(0.94), in: .rect(cornerRadius: 22, style: .continuous))
        .overlay(alignment: .leading) {
            RoundedRectangle(cornerRadius: 3)
                .fill(JoyPalette.primary)
                .frame(width: 5)
                .padding(.vertical, 18)
        }
        .shadow(color: JoyPalette.ink.opacity(0.07), radius: 12, y: 7)
    }
}

struct AddFeedView: View {
    @Environment(\.dismiss) private var dismiss
    @Environment(SessionStore.self) private var session
    @State private var url = ""
    @State private var preview: FeedPreview?
    @State private var busy = false
    let onAdded: () async -> Void

    var body: some View {
        NavigationStack {
            Form {
                Section("Feed URL") {
                    TextField("https://example.com/feed.xml", text: $url)
                        .keyboardType(.URL).textInputAutocapitalization(.never).autocorrectionDisabled()
                    Button("Preview") { Task { await runPreview() } }.disabled(url.isEmpty || busy)
                }
                if let preview {
                    Section("Preview") {
                        LabeledContent("Title", value: preview.title)
                        if !preview.description.isEmpty { Text(preview.description) }
                    }
                }
            }
            .navigationTitle("Add Feed")
            .toolbar {
                ToolbarItem(placement: .cancellationAction) { Button("Cancel") { dismiss() } }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Add") { Task { await add() } }.disabled(url.isEmpty || busy)
                }
            }
        }
        .presentationDetents([.medium, .large])
    }

    private func runPreview() async {
        busy = true; defer { busy = false }
        do { preview = try await session.api.previewFeed(url: url) } catch { session.report(error) }
    }

    private func add() async {
        busy = true; defer { busy = false }
        do {
            _ = try await session.api.addFeed(url: url)
            await onAdded()
            dismiss()
        } catch { session.report(error) }
    }
}
