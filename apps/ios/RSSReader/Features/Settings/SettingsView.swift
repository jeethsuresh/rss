import SwiftUI

struct SettingsView: View {
    @Environment(SessionStore.self) private var session
    @State private var info: SystemInfo?

    private var user: User? {
        if case let .signedIn(user) = session.phase { user } else { nil }
    }

    var body: some View {
        NavigationStack {
            List {
                VStack(alignment: .leading, spacing: 14) {
                    HStack(spacing: 14) {
                        Image("BrandMark")
                            .resizable()
                            .scaledToFit()
                            .frame(width: 68, height: 68)
                            .clipShape(.rect(cornerRadius: 17, style: .continuous))
                        VStack(alignment: .leading, spacing: 4) {
                            Text("Your reader, your rhythm")
                                .font(.system(.title3, design: .rounded, weight: .black))
                            Text(user?.username ?? "RSS Reader")
                                .foregroundStyle(.secondary)
                        }
                    }
                    if let server = session.serverURL {
                        Label(server.host() ?? server.absoluteString, systemImage: "checkmark.circle.fill")
                            .font(.caption.weight(.bold))
                            .foregroundStyle(JoyPalette.mint)
                    }
                }
                .padding(18)
                .frame(maxWidth: .infinity, alignment: .leading)
                .background(JoyPalette.primary.opacity(0.14), in: .rect(cornerRadius: 24, style: .continuous))
                .joyfulListRow()

                Section("Reading") {
                    NavigationLink { PreferencesView() } label: {
                        Label("Preferences", systemImage: "textformat").foregroundStyle(JoyPalette.violet)
                    }
                }
                Section("Feeds") {
                    NavigationLink { FeedManagementView() } label: { Label("Feeds", systemImage: "dot.radiowaves.left.and.right").foregroundStyle(JoyPalette.cyan) }
                    NavigationLink { FolderManagementView() } label: { Label("Folders", systemImage: "folder.fill").foregroundStyle(JoyPalette.sunflower) }
                    NavigationLink { AISettingsView() } label: { Label("Stories & AI", systemImage: "sparkles").foregroundStyle(JoyPalette.coral) }
                }
                Section("Server") {
                    LabeledContent("Account", value: user?.username ?? "")
                    if let server = session.serverURL { LabeledContent("Address", value: server.absoluteString) }
                    if let info {
                        LabeledContent("Version", value: info.version)
                        LabeledContent("Protocol", value: String(info.protocolVersion))
                    }
                    Button("Sign out", systemImage: "rectangle.portrait.and.arrow.right", role: .destructive) {
                        Task { await session.signOut() }
                    }
                }
                Section {
                    Text("This iOS app stores no feed database and runs no background crawlers. Content, sports data, AI processing, and reading state are owned by your server.")
                        .font(.caption).foregroundStyle(.secondary)
                }
            }
            .scrollContentBackground(.hidden)
            .background { JoyfulBackdrop() }
            .navigationTitle("Settings")
            .navigationBarTitleDisplayMode(.inline)
            .task {
                do { info = try await session.api.systemInfo() } catch { session.report(error) }
            }
        }
    }
}

private struct PreferencesView: View {
    @Environment(SessionStore.self) private var session
    @State private var settings: ReaderSettings?
    @State private var saving = false

    var body: some View {
        Form {
            if let binding = Binding($settings) {
                Section("Appearance") {
                    Picker("Theme", selection: binding.theme) {
                        Text("System").tag("system"); Text("Light").tag("light"); Text("Dark").tag("dark")
                    }
                    Picker("Article density", selection: binding.articleDensity) {
                        Text("Comfortable").tag("comfortable"); Text("Compact").tag("compact")
                    }
                }
                Section("Reading") {
                    Picker("Default sort", selection: binding.defaultSort) {
                        Text("Newest first").tag("newest"); Text("Oldest first").tag("oldest")
                    }
                    Toggle("Mark read when opened", isOn: binding.markReadOnOpen)
                    Toggle("Notifications", isOn: binding.notificationsEnabled)
                }
                Section("Polling") {
                    Stepper("Default: \(binding.defaultPollIntervalSeconds.wrappedValue / 60) min", value: binding.defaultPollIntervalSeconds, in: 60...86400, step: 60)
                }
                Section { Button("Save changes") { Task { await save() } }.disabled(saving) }
            } else {
                HStack { Spacer(); ProgressView(); Spacer() }
            }
        }
        .navigationTitle("Preferences")
        .task { await load() }
    }

    private func load() async {
        do { settings = try await session.api.settings() } catch { session.report(error) }
    }
    private func save() async {
        guard let settings else { return }
        saving = true; defer { saving = false }
        do { self.settings = try await session.api.updateSettings(settings) } catch { session.report(error) }
    }
}

private struct FeedManagementView: View {
    @Environment(SessionStore.self) private var session
    @State private var feeds: [Feed] = []
    @State private var importText = ""
    @State private var exportText = ""
    @State private var showingImport = false

    var body: some View {
        List {
            Section {
                Button("Refresh all", systemImage: "arrow.clockwise") { Task { await refreshAll() } }
                Button("Import URLs", systemImage: "square.and.arrow.down") { showingImport = true }
                if !exportText.isEmpty {
                    ShareLink(item: exportText, preview: SharePreview("RSS feed URLs")) {
                        Label("Export URLs", systemImage: "square.and.arrow.up")
                    }
                } else {
                    Button("Prepare export", systemImage: "square.and.arrow.up") { Task { await prepareExport() } }
                }
            }
            Section("Subscriptions") {
                ForEach(feeds.filter { !$0.isReadLater }) { feed in
                    NavigationLink { FeedSettingsView(feed: feed) { updated in replace(updated) } } label: {
                        HStack(spacing: 12) {
                            FeedIconView(url: URL(string: feed.iconUrl), title: feed.title, size: 42)
                            VStack(alignment: .leading) {
                                HStack { Text(feed.title).font(.headline); Spacer(); Text(String(feed.unreadCount)).foregroundStyle(JoyPalette.violet).fontWeight(.bold) }
                                Text(feed.url).font(.caption).foregroundStyle(.secondary).lineLimit(1)
                                if !feed.lastError.isEmpty { Text(feed.lastError).font(.caption).foregroundStyle(.red).lineLimit(2) }
                            }
                        }
                        .padding(.vertical, 4)
                    }
                    .swipeActions {
                        Button("Delete", systemImage: "trash", role: .destructive) { Task { await remove(feed) } }
                        Button("Refresh", systemImage: "arrow.clockwise") { Task { await refresh(feed) } }.tint(.blue)
                    }
                }
            }
        }
        .navigationTitle("Feeds")
        .refreshable { await load() }
        .task { await load() }
        .sheet(isPresented: $showingImport) {
            NavigationStack {
                Form { TextEditor(text: $importText).frame(minHeight: 220) }
                    .navigationTitle("Import Feed URLs")
                    .toolbar {
                        ToolbarItem(placement: .cancellationAction) { Button("Cancel") { showingImport = false } }
                        ToolbarItem(placement: .confirmationAction) { Button("Import") { Task { await runImport() } } }
                    }
            }
        }
    }

    private func load() async { do { feeds = try await session.api.listFeeds() } catch { session.report(error) } }
    private func refreshAll() async { do { try await session.api.refreshAllFeeds() } catch { session.report(error) } }
    private func prepareExport() async { do { exportText = try await session.api.exportFeedURLs() } catch { session.report(error) } }
    private func runImport() async {
        do { _ = try await session.api.importFeedURLs(importText); showingImport = false; await load() } catch { session.report(error) }
    }
    private func refresh(_ feed: Feed) async { do { try await session.api.refreshFeed(id: feed.id) } catch { session.report(error) } }
    private func remove(_ feed: Feed) async {
        do { try await session.api.removeFeed(id: feed.id); feeds.removeAll { $0.id == feed.id } } catch { session.report(error) }
    }
    private func replace(_ feed: Feed) { if let index = feeds.firstIndex(where: { $0.id == feed.id }) { feeds[index] = feed } }
}

private struct FeedSettingsView: View {
    @Environment(SessionStore.self) private var session
    @State private var feed: Feed
    let onChange: (Feed) -> Void

    init(feed: Feed, onChange: @escaping (Feed) -> Void) { _feed = State(initialValue: feed); self.onChange = onChange }

    var body: some View {
        Form {
            Section {
                LabeledContent("URL", value: feed.url)
                Toggle("Enabled", isOn: Binding(get: { feed.enabled }, set: { value in Task { await setEnabled(value) } }))
                Picker("Polling interval", selection: Binding(get: { feed.pollIntervalSeconds }, set: { value in Task { await setPoll(value) } })) {
                    Text("5 minutes").tag(300); Text("15 minutes").tag(900); Text("1 hour").tag(3600); Text("6 hours").tag(21600); Text("1 day").tag(86400)
                }
            }
            Section("Server health") {
                LabeledContent("Crawl attempts", value: String(feed.crawlAttempts))
                LabeledContent("Crawl failures", value: String(feed.crawlFailures))
                LabeledContent("Bad crawl rate", value: feed.badCrawlPercent.formatted(.percent.precision(.fractionLength(1))))
                if let last = feed.lastSuccessAt?.serverDate { LabeledContent("Last success", value: last.formatted(date: .abbreviated, time: .shortened)) }
            }
        }
        .navigationTitle(feed.title)
    }

    private func setEnabled(_ enabled: Bool) async { await update { try await session.api.setFeedEnabled(id: feed.id, enabled: enabled) } }
    private func setPoll(_ seconds: Int) async { await update { try await session.api.setFeedPollInterval(id: feed.id, seconds: seconds) } }
    private func update(_ operation: () async throws -> Feed) async {
        do { feed = try await operation(); onChange(feed) } catch { session.report(error) }
    }
}

private struct FolderManagementView: View {
    @Environment(SessionStore.self) private var session
    @State private var folders: [Folder] = []
    @State private var feeds: [Feed] = []
    @State private var newName = ""

    var body: some View {
        List {
            Section("New folder") {
                HStack {
                    TextField("Name", text: $newName)
                    Button("Add") { Task { await create() } }.disabled(newName.trimmingCharacters(in: .whitespaces).isEmpty)
                }
            }
            ForEach(folders) { folder in
                Section(folder.name) {
                    ForEach(feeds.filter { !$0.isReadLater }) { feed in
                        Toggle(feed.title, isOn: Binding(
                            get: { folder.feedIds.contains(feed.id) },
                            set: { assigned in Task { await assign(folder: folder, feed: feed, assigned: assigned) } }
                        ))
                    }
                    Button("Delete folder", role: .destructive) { Task { await remove(folder) } }
                }
            }
        }
        .navigationTitle("Folders")
        .task { await load() }
    }

    private func load() async {
        do { async let folders = session.api.listFolders(); async let feeds = session.api.listFeeds(); (self.folders, self.feeds) = try await (folders, feeds) }
        catch { session.report(error) }
    }
    private func create() async {
        do { _ = try await session.api.createFolder(name: newName); newName = ""; await load() } catch { session.report(error) }
    }
    private func assign(folder: Folder, feed: Feed, assigned: Bool) async {
        do { try await session.api.assignFeed(folderID: folder.id, feedID: feed.id, assigned: assigned); await load() } catch { session.report(error) }
    }
    private func remove(_ folder: Folder) async {
        do { try await session.api.removeFolder(id: folder.id); await load() } catch { session.report(error) }
    }
}

private struct AISettingsView: View {
    @Environment(SessionStore.self) private var session
    @State private var status: AIStatus?
    @State private var logs: [AILogEntry] = []
    @State private var resultMessage: String?

    var body: some View {
        List {
            if let status {
                Section("Status") {
                    LabeledContent("Running", value: status.running ? "Yes" : "No")
                    LabeledContent("Processed", value: "\(status.processed) / \(status.total)")
                    LabeledContent("Pending", value: String(status.pending))
                    LabeledContent("Failed", value: String(status.failed))
                    if !status.lastError.isEmpty { Text(status.lastError).foregroundStyle(.red) }
                }
            }
            Section("Actions") {
                Button("Test AI connection", systemImage: "bolt.horizontal") { Task { await test() } }
                Menu("Scan articles", systemImage: "sparkles") {
                    Button("Last 24 hours") { Task { await scan("24h") } }
                    Button("Last 7 days") { Task { await scan("7d") } }
                    Button("Missed articles") { Task { await scan("missed") } }
                }
                Button("Retry failed", systemImage: "arrow.clockwise") { Task { await retry() } }
                Button("Rebuild stories", systemImage: "square.stack.3d.up") { Task { await reindex() } }
                if let resultMessage { Text(resultMessage).font(.caption).foregroundStyle(.secondary) }
            }
            Section("Recent log") {
                ForEach(logs) { entry in
                    VStack(alignment: .leading, spacing: 3) {
                        HStack { Text(entry.level.uppercased()).font(.caption.bold()); Spacer(); Text(entry.ts).font(.caption2) }
                        Text(entry.message)
                        if let detail = entry.detail, !detail.isEmpty { Text(detail).font(.caption).foregroundStyle(.secondary) }
                    }
                }
            }
        }
        .navigationTitle("Stories & AI")
        .refreshable { await load() }
        .task(id: session.refreshGeneration) { await load() }
    }

    private func load() async {
        do { async let status = session.api.aiStatus(); async let logs = session.api.aiLogs(); (self.status, self.logs) = try await (status, logs) }
        catch { session.report(error) }
    }
    private func test() async { do { resultMessage = try await session.api.testAI().message } catch { session.report(error) } }
    private func scan(_ window: String) async { do { status = try await session.api.scanAI(window: window).status; resultMessage = "Scan queued." } catch { session.report(error) } }
    private func retry() async { do { let result = try await session.api.retryAI(); status = result.status; resultMessage = "Requeued \(result.requeued) items." } catch { session.report(error) } }
    private func reindex() async { do { resultMessage = "Built \(try await session.api.reindexStories()) stories." } catch { session.report(error) } }
}
