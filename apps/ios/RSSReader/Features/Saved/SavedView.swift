import SwiftUI

struct ReadLaterView: View {
    enum Filter: String, CaseIterable, Identifiable {
        case all, unread, starred, archived
        var id: Self { self }
        var title: String { rawValue.capitalized }
    }

    @Environment(SessionStore.self) private var session
    @State private var filter: Filter = .all
    @State private var search = ""
    @State private var articles: [Article] = []
    @State private var loading = false
    @State private var showingAdd = false

    private struct LoadKey: Hashable { let filter: Filter; let search: String; let generation: Int }

    var body: some View {
        NavigationStack {
            Group {
                if loading && articles.isEmpty {
                    LoadingOverlay(title: "Loading Read Later…")
                } else {
                    List {
                        if articles.isEmpty {
                            ContentUnavailableView("Nothing in Read Later", systemImage: "bookmark", description: Text("Send an article here or add any URL."))
                                .joyfulListRow()
                        }
                        ForEach(articles) { article in
                            NavigationLink(value: article) { ArticleRow(article: article) }
                                .buttonStyle(JoyPressStyle())
                                .swipeActions(edge: .leading, allowsFullSwipe: true) {
                                    Button(article.isRead ? "Unread" : "Read", systemImage: article.isRead ? "circle" : "checkmark") {
                                        Task { await update { try await session.api.setArticleRead(id: article.id, read: !article.isRead) } }
                                    }.tint(.blue)
                                }
                                .swipeActions(edge: .trailing, allowsFullSwipe: false) {
                                    Button("Delete", systemImage: "trash", role: .destructive) {
                                        Task { await remove(article) }
                                    }
                                    Button(article.archivedAt == nil ? "Archive" : "Restore", systemImage: article.archivedAt == nil ? "archivebox" : "arrow.uturn.backward") {
                                        Task { await update { try await session.api.archiveReadLater(id: article.id, archived: article.archivedAt == nil) } }
                                    }.tint(.orange)
                                    Button(article.isStarred ? "Unstar" : "Star", systemImage: "star") {
                                        Task { await update { try await session.api.toggleArticleStar(id: article.id) } }
                                    }.tint(.yellow)
                                }
                                .joyfulListRow()
                        }
                    }
                    .listStyle(.plain)
                    .scrollContentBackground(.hidden)
                    .background { JoyfulBackdrop() }
                }
            }
            .navigationTitle("Read Later")
            .navigationBarTitleDisplayMode(.inline)
            .safeAreaInset(edge: .top) {
                Picker("Filter", selection: $filter) {
                    ForEach(Filter.allCases) { Text($0.title).tag($0) }
                }
                .pickerStyle(.segmented)
                .padding(.horizontal)
                .padding(.vertical, 6)
                .background(.ultraThinMaterial)
            }
            .searchable(text: $search, prompt: "Search Read Later")
            .toolbar { Button("Add URL", systemImage: "plus") { showingAdd = true } }
            .refreshable { await load() }
            .navigationDestination(for: Article.self) { article in
                ReaderView(article: article, originalOnly: true) { replace($0) }
            }
            .sheet(isPresented: $showingAdd) { AddReadLaterURLView { await load() } }
            .task(id: LoadKey(filter: filter, search: search, generation: session.refreshGeneration)) {
                if !search.isEmpty { try? await Task.sleep(for: .milliseconds(300)) }
                guard !Task.isCancelled else { return }
                await load()
            }
        }
    }

    private func load() async {
        loading = true; defer { loading = false }
        do { articles = try await session.api.listReadLater(filter: filter.rawValue, search: search) }
        catch { session.report(error) }
    }

    private func update(_ operation: () async throws -> Article) async {
        do { replace(try await operation()) } catch { session.report(error) }
    }

    private func replace(_ article: Article) {
        if let index = articles.firstIndex(where: { $0.id == article.id }) { articles[index] = article }
    }

    private func remove(_ article: Article) async {
        do {
            try await session.api.removeReadLater(id: article.id)
            articles.removeAll { $0.id == article.id }
        } catch { session.report(error) }
    }
}

private struct AddReadLaterURLView: View {
    @Environment(\.dismiss) private var dismiss
    @Environment(SessionStore.self) private var session
    @State private var url = ""
    @State private var busy = false
    let onAdded: () async -> Void

    var body: some View {
        NavigationStack {
            Form {
                TextField("https://example.com/article", text: $url)
                    .keyboardType(.URL).textInputAutocapitalization(.never).autocorrectionDisabled()
            }
            .navigationTitle("Add to Read Later")
            .toolbar {
                ToolbarItem(placement: .cancellationAction) { Button("Cancel") { dismiss() } }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Add") { Task { await add() } }.disabled(url.isEmpty || busy)
                }
            }
        }
        .presentationDetents([.medium])
    }

    private func add() async {
        busy = true; defer { busy = false }
        do {
            _ = try await session.api.addReadLater(url: url)
            await onAdded()
            dismiss()
        } catch { session.report(error) }
    }
}
