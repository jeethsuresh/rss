import SwiftUI

@main
struct RSSReaderApp: App {
    @State private var session = SessionStore()

    var body: some Scene {
        WindowGroup {
            RootView()
                .environment(session)
                .task { await session.restore() }
                .alert("RSS Reader", isPresented: alertBinding) {
                    Button("OK") { session.alertMessage = nil }
                } message: {
                    Text(session.alertMessage ?? "")
                }
        }
    }

    private var alertBinding: Binding<Bool> {
        Binding(
            get: { session.alertMessage != nil },
            set: { if !$0 { session.alertMessage = nil } }
        )
    }
}

private struct RootView: View {
    @Environment(SessionStore.self) private var session

    var body: some View {
        switch session.phase {
        case .restoring:
            ProgressView("Restoring your session…")
        case .signedOut:
            SignInView()
        case .signedIn:
            ReaderShell()
        }
    }
}

private enum SidebarDestination: Hashable {
    case unread
    case allArticles
    case starred
    case metaStories
    case feed(String)
    case folder(String)
    case readLater
    case sports
    case settings

    var libraryScope: LibraryStore.Scope? {
        switch self {
        case .unread: .unread
        case .allArticles: .all
        case .starred: .starred
        case .metaStories: .stories
        case let .feed(id): .feed(id)
        case let .folder(id): .folder(id)
        case .readLater, .sports, .settings: nil
        }
    }
}

private struct ReaderShell: View {
    @Environment(SessionStore.self) private var session
    @State private var selection: SidebarDestination? = .unread
    @State private var feeds: [Feed] = []
    @State private var folders: [Folder] = []
    @State private var showingAddFeed = false

    private var visibleFeeds: [Feed] {
        feeds.filter { !$0.isReadLater }.sorted { $0.title.localizedStandardCompare($1.title) == .orderedAscending }
    }

    private var assignedFeedIDs: Set<String> {
        Set(folders.flatMap(\.feedIds))
    }

    private var unfiledFeeds: [Feed] {
        visibleFeeds.filter { !assignedFeedIDs.contains($0.id) }
    }

    var body: some View {
        NavigationSplitView {
            List(selection: $selection) {
                Section {
                    sidebarLink("Unread", symbol: "circle.fill", count: visibleFeeds.reduce(0) { $0 + $1.unreadCount }, destination: .unread)
                    sidebarLink("All Articles", symbol: "tray.full.fill", destination: .allArticles)
                    sidebarLink("Starred", symbol: "star.fill", destination: .starred)
                    sidebarLink("Meta-stories", symbol: "square.stack.3d.up.fill", destination: .metaStories)
                }

                Section("Feeds") {
                    ForEach(folders) { folder in
                        DisclosureGroup {
                            NavigationLink(value: SidebarDestination.folder(folder.id)) {
                                Label("All in \(folder.name)", systemImage: "tray.full")
                            }
                            ForEach(visibleFeeds.filter { folder.feedIds.contains($0.id) }) { feed in
                                feedLink(feed)
                            }
                        } label: {
                            Label(folder.name, systemImage: "folder.fill")
                                .font(.subheadline.weight(.semibold))
                        }
                    }

                    ForEach(unfiledFeeds) { feed in
                        feedLink(feed)
                    }
                }

                Section {
                    sidebarLink("Read Later", symbol: "bookmark.fill", destination: .readLater)
                    sidebarLink("Sports", symbol: "trophy.fill", destination: .sports)
                }
            }
            .navigationTitle("Feeds")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                Button("Add Feed", systemImage: "plus") { showingAddFeed = true }
            }
            .safeAreaInset(edge: .bottom, spacing: 0) {
                Button {
                    selection = .settings
                } label: {
                    Label("Settings", systemImage: "gearshape.fill")
                        .font(.body.weight(.semibold))
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .padding(.horizontal, 18)
                        .padding(.vertical, 14)
                        .contentShape(.rect)
                }
                .buttonStyle(.plain)
                .background(.bar)
                .overlay(alignment: .top) { Divider() }
            }
        } detail: {
            detail
        }
        .tint(JoyPalette.violet)
        .sheet(isPresented: $showingAddFeed) {
            AddFeedView {
                await loadSidebar()
            }
        }
        .task(id: session.refreshGeneration) { await loadSidebar() }
    }

    @ViewBuilder
    private var detail: some View {
        switch selection ?? .unread {
        case let destination where destination.libraryScope != nil:
            LibraryView(scope: destination.libraryScope!)
                .id(destination)
        case .readLater:
            ReadLaterView()
        case .sports:
            SportsView()
        case .settings:
            SettingsView()
        default:
            LibraryView(scope: .unread)
        }
    }

    private func sidebarLink(_ title: String, symbol: String, count: Int? = nil, destination: SidebarDestination) -> some View {
        NavigationLink(value: destination) {
            Label {
                HStack {
                    Text(title)
                    Spacer()
                    if let count, count > 0 {
                        Text(count, format: .number)
                            .font(.caption.monospacedDigit())
                            .foregroundStyle(.secondary)
                    }
                }
            } icon: {
                Image(systemName: symbol)
            }
        }
    }

    private func feedLink(_ feed: Feed) -> some View {
        NavigationLink(value: SidebarDestination.feed(feed.id)) {
            HStack(spacing: 10) {
                FeedIconView(url: URL(string: feed.iconUrl), title: feed.title, size: 24)
                Text(feed.title)
                    .lineLimit(1)
                Spacer()
                if feed.unreadCount > 0 {
                    Text(feed.unreadCount, format: .number)
                        .font(.caption.monospacedDigit())
                        .foregroundStyle(.secondary)
                }
            }
        }
    }

    private func loadSidebar() async {
        do {
            async let feeds = session.api.listFeeds()
            async let folders = session.api.listFolders()
            (self.feeds, self.folders) = try await (feeds, folders)
        } catch {
            session.report(error)
        }
    }
}
