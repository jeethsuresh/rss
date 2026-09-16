import SwiftUI

struct SportsView: View {
    enum League: String, CaseIterable, Identifiable {
        case mlb = "MLB"
        case f1 = "Formula 1"
        var id: Self { self }
    }

    @State private var league: League = .mlb
    @Namespace private var leagueIndicator

    var body: some View {
        NavigationStack {
            Group {
                switch league {
                case .mlb: MLBDashboardView()
                case .f1: F1DashboardView()
                }
            }
            .id(league)
            .transition(.asymmetric(
                insertion: .move(edge: .trailing).combined(with: .opacity),
                removal: .move(edge: .leading).combined(with: .opacity)
            ))
            .navigationTitle("Sports")
            .navigationBarTitleDisplayMode(.inline)
            .safeAreaInset(edge: .top) {
                HStack(spacing: 7) {
                    leagueButton(.mlb, symbol: "baseball.fill")
                    leagueButton(.f1, symbol: "flag.checkered.2.crossed")
                }
                .padding(5)
                .background(.regularMaterial, in: .capsule)
                .padding(.horizontal)
                .padding(.vertical, 7)
                .background(.ultraThinMaterial)
            }
            .animation(.snappy(duration: 0.35, extraBounce: 0.06), value: league)
            .sensoryFeedback(.selection, trigger: league)
        }
    }

    private func leagueButton(_ value: League, symbol: String) -> some View {
        Button { league = value } label: {
            Label(value.rawValue, systemImage: symbol)
                .font(.subheadline.weight(.bold))
                .foregroundStyle(league == value ? .white : .primary)
                .frame(maxWidth: .infinity)
                .padding(.vertical, 9)
                .background {
                    if league == value {
                        Capsule()
                            .fill(value == .mlb ? JoyPalette.coral : JoyPalette.ink)
                            .matchedGeometryEffect(id: "league", in: leagueIndicator)
                    }
                }
        }
        .buttonStyle(.plain)
    }
}

private struct PlayerLeader: Identifiable {
    enum Kind { case batting, pitching }
    let team: MlbTeam
    let player: MlbRosterPlayer
    let kind: Kind
    var id: String { "\(team.id)-\(player.playerId)-\(kind == .batting ? "bat" : "pitch")" }
}

private struct MLBDashboardView: View {
    @Environment(SessionStore.self) private var session
    @State private var games: [MlbGame] = []
    @State private var teams: [MlbTeam] = []
    @State private var followed: [Int] = []
    @State private var season: Int?
    @State private var standings: MlbStandings?
    @State private var playerLeaders: [PlayerLeader] = []
    @State private var loading = false
    @State private var showingTeams = false

    var body: some View {
        List {
            if !followedTeams.isEmpty {
                Section("Tracked teams") {
                    ForEach(followedTeams) { team in
                        NavigationLink { MLBTeamView(team: team, season: season) } label: {
                            TrackedTeamCard(
                                team: team,
                                standing: standingRow(for: team),
                                latestGame: latestGame(for: team)
                            )
                        }
                        .buttonStyle(JoyPressStyle())
                        .joyfulListRow()
                    }
                }
            }

            Section("Latest games") {
                if loading && games.isEmpty {
                    HStack { Spacer(); ProgressView(); Spacer() }
                } else if games.isEmpty {
                    ContentUnavailableView("No recent games", systemImage: "baseball")
                }
                ForEach(games) { game in
                    NavigationLink { MLBGameView(game: game) } label: { ScoreRow(game: game) }
                        .buttonStyle(JoyPressStyle())
                        .joyfulListRow()
                }
            }

            Section("League standings") {
                if let standings {
                    ForEach(standings.sections.filter { $0.kind != "wildcard" }) { section in
                        StandingsPreviewCard(section: section, trackedTeamIDs: Set(followed))
                            .joyfulListRow()
                    }
                    NavigationLink { MLBStandingsView(season: season) } label: {
                        Label("Complete standings", systemImage: "tablecells")
                            .font(.headline)
                    }
                } else if loading {
                    HStack { Spacer(); ProgressView(); Spacer() }
                }
            }

            Section("Player statistics") {
                if playerLeaders.isEmpty && loading {
                    HStack { Spacer(); ProgressView(); Spacer() }
                } else if playerLeaders.isEmpty {
                    ContentUnavailableView(
                        "No player statistics",
                        systemImage: "figure.baseball",
                        description: Text("Follow teams to keep their current leaders here.")
                    )
                } else {
                    ScrollView(.horizontal) {
                        HStack(spacing: 12) {
                            ForEach(playerLeaders) { leader in
                                NavigationLink {
                                    PlayerStatsView(team: leader.team, player: leader.player)
                                } label: {
                                    PlayerStatCard(leader: leader)
                                }
                                .buttonStyle(JoyPressStyle())
                            }
                        }
                        .padding(.vertical, 4)
                    }
                    .scrollIndicators(.hidden)
                    .joyfulListRow()
                }
            }
        }
        .listStyle(.plain)
        .scrollContentBackground(.hidden)
        .background { JoyfulBackdrop() }
        .toolbar {
            Button("Choose teams", systemImage: "person.3") { showingTeams = true }
        }
        .sheet(isPresented: $showingTeams) {
            TeamPickerView(teams: teams, followed: followed) { ids in
                followed = ids
                games = SportsPresentation.latestGames(from: games, trackedTeamIDs: Set(ids))
                Task { await loadPlayerStatistics() }
            }
        }
        .refreshable { await load() }
        .task(id: session.refreshGeneration) { await load() }
    }

    private var followedTeams: [MlbTeam] {
        followed.compactMap { id in teams.first { $0.id == id } }
    }

    private func standingRow(for team: MlbTeam) -> MlbStandingRow? {
        standings?.sections.lazy.flatMap(\.teams).first { $0.team.id == team.id }
    }

    private func latestGame(for team: MlbTeam) -> MlbGame? {
        games.first { $0.awayTeam.id == team.id || $0.homeTeam.id == team.id }
    }

    private func load() async {
        loading = true
        defer { loading = false }
        do {
            async let newTeams = session.api.mlbTeams()
            async let newFollowed = session.api.followedTeams()
            async let seasons = session.api.mlbSeasons()
            async let newStandings = session.api.mlbStandings(season: season)
            let values = try await (newTeams, newFollowed, seasons, newStandings)
            teams = values.0
            followed = values.1
            season = values.2.map(\.seasonId).max()
            standings = values.3
            await loadLatestGames()
            await loadPlayerStatistics()
        } catch {
            session.report(error)
        }
    }

    private func loadLatestGames() async {
        do {
            var recent: [MlbGame] = []
            for offset in 0...3 {
                guard let day = Calendar.current.date(byAdding: .day, value: -offset, to: Date()) else { continue }
                recent.append(contentsOf: try await session.api.mlbDaily(date: day.apiDay))
            }
            games = SportsPresentation.latestGames(from: recent, trackedTeamIDs: Set(followed))
        } catch {
            session.report(error)
        }
    }

    private func loadPlayerStatistics() async {
        guard let season else { return }
        let fallbackTeams = standings?.sections
            .filter { $0.kind != "wildcard" }
            .compactMap { $0.teams.first?.team } ?? []
        var seenTeamIDs = Set<Int>()
        let selectedTeams = (followedTeams.isEmpty ? fallbackTeams : followedTeams)
            .filter { seenTeamIDs.insert($0.id).inserted }
        guard !selectedTeams.isEmpty else {
            playerLeaders = []
            return
        }

        var hitters: [PlayerLeader] = []
        var pitchers: [PlayerLeader] = []
        for team in selectedTeams {
            do {
                let roster = try await session.api.mlbRoster(teamID: team.id, season: season)
                hitters.append(contentsOf: roster.players.filter { $0.batting != nil }.map {
                    PlayerLeader(team: team, player: $0, kind: .batting)
                })
                pitchers.append(contentsOf: roster.players.filter { $0.pitching != nil }.map {
                    PlayerLeader(team: team, player: $0, kind: .pitching)
                })
            } catch {
                continue
            }
        }
        hitters.sort {
            let left = $0.player.batting
            let right = $1.player.batting
            return (left?.homeRuns ?? 0, left?.rbi ?? 0) > (right?.homeRuns ?? 0, right?.rbi ?? 0)
        }
        pitchers.sort {
            let left = Double($0.player.pitching?.era ?? "999") ?? 999
            let right = Double($1.player.pitching?.era ?? "999") ?? 999
            return (left, -($0.player.pitching?.strikeOuts ?? 0)) < (right, -($1.player.pitching?.strikeOuts ?? 0))
        }
        playerLeaders = Array(hitters.prefix(4)) + Array(pitchers.prefix(4))
    }
}

private struct TrackedTeamCard: View {
    let team: MlbTeam
    let standing: MlbStandingRow?
    let latestGame: MlbGame?

    var body: some View {
        HStack(spacing: 14) {
            TeamLogoView(team: team, size: 56)
            VStack(alignment: .leading, spacing: 7) {
                HStack {
                    Text(team.name).font(.headline)
                    Spacer()
                    if let standing {
                        Text("#\(standing.rank)")
                            .font(.caption.weight(.black))
                            .foregroundStyle(JoyPalette.coral)
                    }
                }
                if let standing {
                    HStack(spacing: 10) {
                        Text("\(standing.wins)–\(standing.losses)")
                        Text(standing.winningPercentage)
                        Text(standing.gamesBack == "-" ? "Division leader" : "\(standing.gamesBack) GB")
                    }
                    .font(.caption.monospacedDigit())
                    .foregroundStyle(.secondary)
                }
                if let latestGame {
                    HStack(spacing: 5) {
                        Image(systemName: latestGame.status == "live" ? "dot.radiowaves.left.and.right" : "clock.arrow.circlepath")
                        Text(gameSummary(latestGame))
                            .lineLimit(1)
                    }
                    .font(.caption.weight(.semibold))
                    .foregroundStyle(latestGame.status == "live" ? JoyPalette.coral : .secondary)
                }
            }
            Image(systemName: "chevron.right")
                .font(.caption.bold())
                .foregroundStyle(.tertiary)
        }
        .padding(14)
        .background(.background, in: .rect(cornerRadius: 20))
        .overlay { RoundedRectangle(cornerRadius: 20).stroke(JoyPalette.coral.opacity(0.20)) }
    }

    private func gameSummary(_ game: MlbGame) -> String {
        let opponent = game.awayTeam.id == team.id ? game.homeTeam.abbreviation : game.awayTeam.abbreviation
        let venue = game.awayTeam.id == team.id ? "at" : "vs"
        if let own = game.awayTeam.id == team.id ? game.awayScore : game.homeScore,
           let other = game.awayTeam.id == team.id ? game.homeScore : game.awayScore {
            return "\(game.status.capitalized) · \(venue) \(opponent) · \(own)–\(other)"
        }
        return "\(game.status.replacingOccurrences(of: "_", with: " ").capitalized) · \(venue) \(opponent)"
    }
}

private struct StandingsPreviewCard: View {
    let section: MlbStandingSection
    let trackedTeamIDs: Set<Int>

    private var visibleRows: [MlbStandingRow] {
        var seen = Set<Int>()
        let tracked = section.teams.filter { trackedTeamIDs.contains($0.team.id) }
        return (tracked + Array(section.teams.prefix(3))).filter { seen.insert($0.team.id).inserted }
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack {
                Text(section.name).font(.headline)
                Spacer()
                Text(section.league).font(.caption.bold()).foregroundStyle(JoyPalette.coral)
            }
            ForEach(visibleRows) { row in
                HStack(spacing: 9) {
                    Text(row.rank, format: .number).foregroundStyle(.secondary).frame(width: 18)
                    TeamLogoView(team: row.team, size: 28)
                    Text(row.team.abbreviation).font(.subheadline.bold())
                    if trackedTeamIDs.contains(row.team.id) {
                        Text("TRACKED")
                            .font(.system(size: 8, weight: .black))
                            .foregroundStyle(JoyPalette.coral)
                    }
                    Spacer()
                    Text("\(row.wins)–\(row.losses)").monospacedDigit()
                    Text(row.winningPercentage).foregroundStyle(.secondary).monospacedDigit()
                }
                .font(.subheadline)
            }
        }
        .padding(14)
        .background(.background, in: .rect(cornerRadius: 18))
        .overlay { RoundedRectangle(cornerRadius: 18).stroke(.primary.opacity(0.07)) }
    }
}

private struct PlayerStatsView: View {
    let team: MlbTeam
    let player: MlbRosterPlayer

    var body: some View {
        List {
            Section {
                HStack(spacing: 14) {
                    TeamLogoView(team: team, size: 54)
                    VStack(alignment: .leading, spacing: 4) {
                        Text(player.name).font(.title3.bold())
                        Text([player.position, player.jerseyNumber.map { "#\($0)" }, player.statusDescription]
                            .compactMap { $0 }.joined(separator: " · "))
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                }
                .padding(.vertical, 4)
            }

            if let batting = player.batting {
                Section("Batting") {
                    stat("Games", batting.games)
                    if let value = batting.plateAppearances { stat("Plate appearances", value) }
                    stat("At bats", batting.atBats)
                    stat("Hits", batting.hits)
                    stat("Runs", batting.runs)
                    if let value = batting.doubles { stat("Doubles", value) }
                    if let value = batting.triples { stat("Triples", value) }
                    stat("Home runs", batting.homeRuns)
                    stat("RBI", batting.rbi)
                    stat("Walks", batting.walks)
                    stat("Strikeouts", batting.strikeOuts)
                    if let value = batting.stolenBases { stat("Stolen bases", value) }
                }
                Section("Rate statistics") {
                    stat("AVG", batting.average)
                    stat("OBP", batting.onBasePercentage)
                    stat("SLG", batting.slugging)
                    if let value = batting.ops { stat("OPS", value) }
                }
            }

            if let pitching = player.pitching {
                Section("Pitching") {
                    stat("Games", pitching.games)
                    stat("Starts", pitching.gamesStarted)
                    stat("Record", "\(pitching.wins)–\(pitching.losses)")
                    stat("Saves", pitching.saves)
                    stat("Innings", pitching.inningsPitched)
                    stat("ERA", pitching.era)
                    stat("WHIP", pitching.whip)
                    stat("Strikeouts", pitching.strikeOuts)
                    stat("Walks", pitching.walks)
                }
            }
        }
        .navigationTitle("Player Statistics")
        .navigationBarTitleDisplayMode(.inline)
    }

    private func stat(_ label: String, _ value: Int) -> some View {
        stat(label, String(value))
    }

    private func stat(_ label: String, _ value: String) -> some View {
        LabeledContent(label) {
            Text(value).font(.body.monospacedDigit().weight(.semibold))
        }
    }
}

private struct PlayerStatCard: View {
    let leader: PlayerLeader

    var body: some View {
        VStack(alignment: .leading, spacing: 9) {
            HStack {
                TeamLogoView(team: leader.team, size: 30)
                Spacer()
                Text(leader.kind == .batting ? "HITTING" : "PITCHING")
                    .font(.caption2.weight(.black))
                    .foregroundStyle(leader.kind == .batting ? JoyPalette.coral : JoyPalette.violet)
            }
            Text(leader.player.name)
                .font(.headline)
                .lineLimit(1)
            if let batting = leader.player.batting {
                statLine("AVG", batting.average, "HR", String(batting.homeRuns), "RBI", String(batting.rbi))
            } else if let pitching = leader.player.pitching {
                statLine("ERA", pitching.era, "SO", String(pitching.strikeOuts), "W", String(pitching.wins))
            }
        }
        .padding(14)
        .frame(width: 220, height: 126, alignment: .leading)
        .background(.background, in: .rect(cornerRadius: 20))
        .overlay { RoundedRectangle(cornerRadius: 20).stroke(.primary.opacity(0.07)) }
    }

    private func statLine(_ a: String, _ av: String, _ b: String, _ bv: String, _ c: String, _ cv: String) -> some View {
        HStack {
            stat(a, av)
            Spacer()
            stat(b, bv)
            Spacer()
            stat(c, cv)
        }
    }

    private func stat(_ label: String, _ value: String) -> some View {
        VStack(alignment: .leading, spacing: 1) {
            Text(label).font(.caption2).foregroundStyle(.secondary)
            Text(value).font(.headline.monospacedDigit())
        }
    }
}

private struct TeamPickerView: View {
    @Environment(\.dismiss) private var dismiss
    @Environment(SessionStore.self) private var session
    let teams: [MlbTeam]
    @State var followed: [Int]
    let onChange: ([Int]) -> Void

    var body: some View {
        NavigationStack {
            List(teams) { team in
                Button {
                    if followed.contains(team.id) { followed.removeAll { $0 == team.id } }
                    else { followed.append(team.id) }
                } label: {
                    HStack {
                        TeamLogoView(team: team, size: 42)
                        VStack(alignment: .leading) {
                            Text(team.name).font(.headline).foregroundStyle(.primary)
                            if let league = team.league { Text(league).font(.caption).foregroundStyle(.secondary) }
                        }
                        Spacer()
                        if followed.contains(team.id) {
                            Image(systemName: "checkmark.circle.fill")
                                .font(.title2)
                                .foregroundStyle(JoyPalette.mint)
                        }
                    }
                    .padding(.vertical, 4)
                }
            }
            .navigationTitle("Follow Teams")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) { Button("Cancel") { dismiss() } }
                ToolbarItem(placement: .confirmationAction) { Button("Done") { Task { await save() } } }
            }
        }
    }

    private func save() async {
        do {
            let ids = try await session.api.setFollowedTeams(followed)
            onChange(ids)
            dismiss()
        } catch { session.report(error) }
    }
}

private struct MLBTeamView: View {
    @Environment(SessionStore.self) private var session
    let team: MlbTeam
    let season: Int?
    @State private var games: [MlbGame] = []
    @State private var roster: [MlbRosterPlayer] = []

    var body: some View {
        List {
            Section {
                HStack(spacing: 14) {
                    TeamLogoView(team: team, size: 58)
                    VStack(alignment: .leading, spacing: 3) {
                        Text(team.name).font(.title3.bold())
                        Text("Scores and player statistics").font(.caption).foregroundStyle(.secondary)
                    }
                }
                .padding(.vertical, 5)
            }

            Section("Recent and upcoming") {
                ForEach(relevantGames) { game in
                    NavigationLink { MLBGameView(game: game) } label: { ScoreRow(game: game) }
                        .buttonStyle(JoyPressStyle())
                        .joyfulListRow()
                }
            }

            Section("Player statistics") {
                ForEach(roster) { player in
                    NavigationLink { PlayerStatsView(team: team, player: player) } label: {
                        VStack(alignment: .leading, spacing: 6) {
                            HStack {
                                Text(player.jerseyNumber.map { "#\($0)" } ?? "–")
                                    .frame(width: 38, alignment: .leading)
                                    .foregroundStyle(.secondary)
                                Text(player.name).font(.headline)
                                Spacer()
                                Text(player.position ?? "").font(.caption).foregroundStyle(.secondary)
                            }
                            if let batting = player.batting {
                                Text("AVG \(batting.average)   HR \(batting.homeRuns)   RBI \(batting.rbi)")
                                    .font(.caption.monospacedDigit()).foregroundStyle(.secondary)
                            } else if let pitching = player.pitching {
                                Text("ERA \(pitching.era)   SO \(pitching.strikeOuts)   WHIP \(pitching.whip)")
                                    .font(.caption.monospacedDigit()).foregroundStyle(.secondary)
                            }
                        }
                        .padding(.vertical, 3)
                    }
                }
            }
        }
        .listStyle(.plain)
        .navigationTitle(team.name)
        .navigationBarTitleDisplayMode(.inline)
        .task {
            do {
                async let schedule = session.api.mlbSchedule(teamID: team.id, season: season)
                async let roster = session.api.mlbRoster(teamID: team.id, season: season)
                let values = try await (schedule, roster)
                games = values.0
                self.roster = values.1.players
            } catch { session.report(error) }
        }
    }

    private var relevantGames: [MlbGame] {
        let ordered = games.sorted { ($0.gameDate.serverDate ?? .distantPast) < ($1.gameDate.serverDate ?? .distantPast) }
        let completed = ordered.filter { $0.status == "final" }.suffix(3)
        let upcoming = ordered.filter { $0.status == "live" || $0.status == "scheduled" || $0.status == "pre_game" }.prefix(3)
        return Array(completed) + Array(upcoming)
    }
}

private struct MLBGameView: View {
    @Environment(SessionStore.self) private var session
    let game: MlbGame
    @State private var detail: MlbGameDetail?

    var body: some View {
        List {
            ScoreRow(game: detail?.game ?? game).joyfulListRow()
            if let detail {
                Section("Team totals") {
                    HStack {
                        Text("Team").font(.caption.bold()).foregroundStyle(.secondary)
                        Spacer()
                        Text("R").frame(width: 32)
                        Text("H").frame(width: 32)
                        Text("E").frame(width: 32)
                    }
                    teamTotal(
                        detail.game.awayTeam,
                        runs: detail.game.awayScore,
                        hits: detail.awayHits,
                        errors: detail.awayErrors
                    )
                    teamTotal(
                        detail.game.homeTeam,
                        runs: detail.game.homeScore,
                        hits: detail.homeHits,
                        errors: detail.homeErrors
                    )
                }
                Section("Innings") {
                    ForEach(detail.innings) { inning in
                        LabeledContent("Inning \(inning.number)", value: "\(inning.awayRuns) – \(inning.homeRuns)")
                    }
                }
                if detail.awayBox != nil || detail.homeBox != nil {
                    Section("Box score") {
                        if let box = detail.awayBox { TeamBoxScoreView(box: box) }
                        if let box = detail.homeBox { TeamBoxScoreView(box: box) }
                    }
                }
                Section("Play by play") {
                    ForEach(detail.plays.reversed()) { play in
                        VStack(alignment: .leading, spacing: 4) {
                            Text("\(play.half.capitalized) \(play.inning) · \(play.event)")
                                .font(.caption.bold()).foregroundStyle(.secondary)
                            Text(play.description)
                        }
                    }
                }
            }
        }
        .listStyle(.plain)
        .navigationTitle("\(game.awayTeam.abbreviation) at \(game.homeTeam.abbreviation)")
        .navigationBarTitleDisplayMode(.inline)
        .refreshable { await load() }
        .task(id: session.refreshGeneration) { await load() }
        .onDisappear { Task { try? await session.api.unwatchMlbGame(id: game.id) } }
    }

    private func load() async {
        do { detail = try await session.api.watchMlbGame(id: game.id) }
        catch { session.report(error) }
    }

    private func teamTotal(_ team: MlbTeam, runs: Int?, hits: Int?, errors: Int?) -> some View {
        HStack {
            TeamLogoView(team: team, size: 28)
            Text(team.abbreviation).font(.headline)
            Spacer()
            Text(runs.map(String.init) ?? "–").frame(width: 32)
            Text(hits.map(String.init) ?? "–").frame(width: 32)
            Text(errors.map(String.init) ?? "–").frame(width: 32)
        }
        .monospacedDigit()
    }
}

private struct TeamBoxScoreView: View {
    let box: MlbTeamBox

    var body: some View {
        DisclosureGroup {
            if !box.batters.isEmpty {
                Text("BATTING")
                    .font(.caption2.weight(.black))
                    .foregroundStyle(JoyPalette.coral)
                ForEach(box.batters) { batter in
                    VStack(alignment: .leading, spacing: 4) {
                        HStack {
                            Text(batter.name).font(.subheadline.weight(.semibold)).lineLimit(1)
                            Spacer()
                            Text("\(batter.hits)-\(batter.atBats)").monospacedDigit()
                        }
                        HStack(spacing: 10) {
                            Text("R \(batter.runs)")
                            Text("RBI \(batter.rbi)")
                            Text("HR \(batter.homeRuns)")
                            Text("BB \(batter.walks)")
                            Text("K \(batter.strikeOuts)")
                        }
                        .font(.caption2.monospacedDigit())
                        .foregroundStyle(.secondary)
                    }
                    .padding(.vertical, 2)
                }
            }

            if !box.pitchers.isEmpty {
                Text("PITCHING")
                    .font(.caption2.weight(.black))
                    .foregroundStyle(JoyPalette.violet)
                    .padding(.top, 6)
                ForEach(box.pitchers) { pitcher in
                    VStack(alignment: .leading, spacing: 4) {
                        HStack {
                            Text(pitcher.name).font(.subheadline.weight(.semibold)).lineLimit(1)
                            Spacer()
                            Text("\(pitcher.inningsPitched) IP").monospacedDigit()
                        }
                        HStack(spacing: 10) {
                            Text("H \(pitcher.hits)")
                            Text("ER \(pitcher.earnedRuns)")
                            Text("BB \(pitcher.walks)")
                            Text("K \(pitcher.strikeOuts)")
                            Text("P \(pitcher.pitchesThrown)")
                        }
                        .font(.caption2.monospacedDigit())
                        .foregroundStyle(.secondary)
                    }
                    .padding(.vertical, 2)
                }
            }
        } label: {
            HStack(spacing: 10) {
                TeamLogoView(team: box.team, size: 34)
                Text(box.team.name).font(.headline)
            }
        }
    }
}

private struct MLBStandingsView: View {
    @Environment(SessionStore.self) private var session
    let season: Int?
    @State private var standings: MlbStandings?

    var body: some View {
        List {
            ForEach(standings?.sections ?? []) { section in
                Section(section.name) {
                    ForEach(section.teams) { row in
                        HStack {
                            Text(row.rank, format: .number).foregroundStyle(.secondary).frame(width: 24)
                            TeamLogoView(team: row.team, size: 34)
                            Text(row.team.abbreviation).bold()
                            Spacer()
                            Text("\(row.wins)–\(row.losses)").monospacedDigit()
                            Text(row.gamesBack == "-" ? "—" : "\(row.gamesBack) GB")
                                .frame(width: 62, alignment: .trailing).foregroundStyle(.secondary)
                        }
                    }
                }
            }
        }
        .navigationTitle("MLB Standings")
        .navigationBarTitleDisplayMode(.inline)
        .task {
            do { standings = try await session.api.mlbStandings(season: season) }
            catch { session.report(error) }
        }
    }
}

private extension Date {
    var apiDay: String { formatted(.iso8601.year().month().day().dateSeparator(.dash)) }
}
