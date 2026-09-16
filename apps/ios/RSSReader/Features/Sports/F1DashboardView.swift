import SwiftUI

struct F1DashboardView: View {
    @Environment(SessionStore.self) private var session
    @State private var years: [Int] = []
    @State private var year = Calendar.current.component(.year, from: Date())
    @State private var races: [F1Race] = []
    @State private var standings: F1Standings?
    @State private var loading = false

    var body: some View {
        List {
            Section("Championship") {
                if let standings {
                    HStack(alignment: .top, spacing: 12) {
                        ChampionshipPreview(
                            title: "WCC",
                            rows: standings.constructors.prefix(3).map { (String($0.position), $0.teamName, $0.points) },
                            color: JoyPalette.coral
                        )
                        ChampionshipPreview(
                            title: "WDC",
                            rows: standings.drivers.prefix(3).map { (String($0.position), $0.nameAcronym ?? $0.name, $0.points) },
                            color: JoyPalette.violet
                        )
                    }
                    .joyfulListRow()
                    NavigationLink { F1StandingsView(year: year, initial: standings) } label: {
                        Label("Complete WCC and WDC", systemImage: "trophy.fill")
                            .font(.headline)
                    }
                } else if loading {
                    HStack { Spacer(); ProgressView(); Spacer() }
                }
            }

            Section("Previous and next") {
                if let previousRace {
                    NavigationLink { F1RaceView(race: previousRace) } label: {
                        PriorityRaceCard(label: "PREVIOUS", race: previousRace, color: JoyPalette.violet)
                    }
                    .buttonStyle(JoyPressStyle())
                    .joyfulListRow()
                }
                if let nextRace {
                    NavigationLink { F1RaceView(race: nextRace) } label: {
                        PriorityRaceCard(label: "NEXT", race: nextRace, color: JoyPalette.coral)
                    }
                    .buttonStyle(JoyPressStyle())
                    .joyfulListRow()
                }
            }

            Section("Season") {
                Picker("Season", selection: $year) {
                    ForEach(years, id: \.self) { Text(String($0)).tag($0) }
                }
                .font(.headline)
            }

            Section("Full calendar") {
                if loading && races.isEmpty { HStack { Spacer(); ProgressView(); Spacer() } }
                ForEach(orderedRaces) { race in
                    NavigationLink { F1RaceView(race: race) } label: { RaceWeekendCard(race: race) }
                        .buttonStyle(JoyPressStyle())
                        .joyfulListRow()
                }
            }
        }
        .listStyle(.plain)
        .scrollContentBackground(.hidden)
        .background { JoyfulBackdrop() }
        .refreshable { await loadRaces() }
        .task(id: year) { await loadRaces() }
        .task(id: session.refreshGeneration) { await load() }
    }

    private func load() async {
        loading = true; defer { loading = false }
        do {
            years = try await session.api.f1Years().map(\.year).sorted(by: >)
            if !years.contains(year), let latest = years.first { year = latest }
            async let races = session.api.f1Races(year: year)
            async let standings = session.api.f1Standings(year: year)
            (self.races, self.standings) = try await (races, standings)
        } catch { session.report(error) }
    }

    private func loadRaces() async {
        loading = true; defer { loading = false }
        do {
            async let races = session.api.f1Races(year: year)
            async let standings = session.api.f1Standings(year: year)
            (self.races, self.standings) = try await (races, standings)
        } catch { session.report(error) }
    }

    private var orderedRaces: [F1Race] {
        SportsPresentation.orderedRaces(races)
    }

    private var previousRace: F1Race? {
        SportsPresentation.previousRace(in: races)
    }

    private var nextRace: F1Race? {
        SportsPresentation.nextRace(in: races)
    }
}

private struct ChampionshipPreview: View {
    let title: String
    let rows: [(position: String, name: String, points: Double)]
    let color: Color

    var body: some View {
        VStack(alignment: .leading, spacing: 9) {
            Text(title)
                .font(.title3.weight(.black))
                .foregroundStyle(color)
            ForEach(Array(rows.enumerated()), id: \.offset) { _, row in
                HStack(spacing: 6) {
                    Text(row.position).foregroundStyle(.secondary).frame(width: 14)
                    Text(row.name).lineLimit(1)
                    Spacer(minLength: 2)
                    Text(row.points, format: .number).monospacedDigit().bold()
                }
                .font(.caption)
            }
        }
        .padding(13)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(.background, in: .rect(cornerRadius: 18))
        .overlay { RoundedRectangle(cornerRadius: 18).stroke(color.opacity(0.25)) }
    }
}

private struct PriorityRaceCard: View {
    let label: String
    let race: F1Race
    let color: Color

    var body: some View {
        HStack(spacing: 13) {
            CountryFlagView(code: race.countryCode, countryName: race.countryName, size: 38)
            VStack(alignment: .leading, spacing: 4) {
                Text(label).font(.caption2.weight(.black)).tracking(0.8).foregroundStyle(color)
                Text(race.name).font(.headline)
                Text(race.circuitShortName).font(.caption).foregroundStyle(.secondary)
            }
            Spacer()
            if let date = race.dateStart.serverDate {
                Text(date, format: .dateTime.month(.abbreviated).day())
                    .font(.subheadline.bold())
            }
            Image(systemName: "chevron.right").font(.caption.bold()).foregroundStyle(.tertiary)
        }
        .padding(14)
        .background(.background, in: .rect(cornerRadius: 19))
        .overlay(alignment: .leading) { Rectangle().fill(color).frame(width: 4) }
        .clipShape(.rect(cornerRadius: 19))
    }
}

private struct RaceWeekendCard: View {
    let race: F1Race

    var body: some View {
        HStack(spacing: 16) {
            VStack(spacing: 4) {
                CountryFlagView(code: race.countryCode, countryName: race.countryName, size: 40)
                Text((race.countryCode ?? race.countryName.prefix(3).uppercased()))
                    .font(.caption2.weight(.black))
                    .foregroundStyle(.secondary)
            }
            .frame(width: 72, height: 82)
            .background(Color(.secondarySystemGroupedBackground), in: .rect(cornerRadius: 18, style: .continuous))

            VStack(alignment: .leading, spacing: 8) {
                HStack {
                    Text(race.name)
                        .font(.system(.headline, design: .rounded, weight: .bold))
                        .lineLimit(2)
                    Spacer()
                    if let date = race.dateStart.serverDate {
                        VStack(spacing: 0) {
                            Text(date, format: .dateTime.day())
                                .font(.title2.bold())
                            Text(date, format: .dateTime.month(.abbreviated))
                                .font(.caption.weight(.black))
                                .foregroundStyle(JoyPalette.coral)
                        }
                    }
                }
                Label("\(race.location), \(race.countryName)", systemImage: "mappin.and.ellipse")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .lineLimit(1)
                Text(race.circuitShortName.uppercased())
                    .font(.caption2.weight(.black))
                    .tracking(0.8)
                    .foregroundStyle(JoyPalette.violet)
            }
        }
        .padding(14)
        .background(.background.opacity(0.95), in: .rect(cornerRadius: 23, style: .continuous))
        .overlay {
            RoundedRectangle(cornerRadius: 23, style: .continuous)
                .stroke(.primary.opacity(0.06), lineWidth: 1)
        }
        .shadow(color: JoyPalette.ink.opacity(0.07), radius: 12, y: 7)
    }
}

private struct F1RaceView: View {
    @Environment(SessionStore.self) private var session
    let race: F1Race
    @State private var detail: F1RaceDetail?

    var body: some View {
        List {
            HStack(spacing: 16) {
                CountryFlagView(code: race.countryCode, countryName: race.countryName, size: 58)
                VStack(alignment: .leading, spacing: 8) {
                    Text(race.countryName.uppercased())
                        .font(.caption.weight(.black))
                        .tracking(1.2)
                    Text(race.officialName ?? race.name)
                        .font(.system(.title2, design: .rounded, weight: .black))
                    Text("\(race.circuitShortName) · \(race.location)")
                        .font(.subheadline)
                        .foregroundStyle(.secondary)
                }
            }
            .padding(20)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(.background, in: .rect(cornerRadius: 24, style: .continuous))
            .overlay(alignment: .bottom) {
                Rectangle().fill(JoyPalette.coral).frame(height: 4)
            }
            .clipShape(.rect(cornerRadius: 24, style: .continuous))
            .joyfulListRow()

            Section {
                LabeledContent("Circuit", value: race.circuitShortName)
                LabeledContent("Location", value: "\(race.location), \(race.countryName)")
                LabeledContent("Status", value: race.status.replacingOccurrences(of: "_", with: " ").capitalized)
            }
            if let detail {
                if let sessions = detail.sessions ?? race.sessions, !sessions.isEmpty {
                    Section("Race weekend") {
                        ScrollView(.horizontal) {
                            HStack(spacing: 10) {
                                ForEach(sessions) { session in
                                    VStack(alignment: .leading, spacing: 5) {
                                        Image(systemName: session.kind == "race" ? "flag.checkered" : "stopwatch.fill")
                                            .foregroundStyle(session.kind == "race" ? JoyPalette.coral : JoyPalette.violet)
                                        Text(session.sessionName)
                                            .font(.caption.weight(.bold))
                                            .lineLimit(1)
                                        if let date = session.dateStart.serverDate {
                                            Text(date, format: .dateTime.weekday(.abbreviated).hour().minute())
                                                .font(.caption2)
                                                .foregroundStyle(.secondary)
                                        }
                                    }
                                    .padding(12)
                                    .frame(width: 126, alignment: .leading)
                                    .background(.background, in: .rect(cornerRadius: 16, style: .continuous))
                                    .overlay { RoundedRectangle(cornerRadius: 16).stroke(.primary.opacity(0.07)) }
                                }
                            }
                            .padding(.vertical, 3)
                        }
                        .scrollIndicators(.hidden)
                        .joyfulListRow()
                    }
                }
                if !detail.results.isEmpty {
                    Section("Podium") {
                        HStack(alignment: .bottom, spacing: 10) {
                            ForEach(Array(detail.results.sorted { $0.position < $1.position }.prefix(3))) { result in
                                VStack(spacing: 7) {
                                    Image(systemName: result.position == 1 ? "trophy.fill" : "medal.fill")
                                        .font(result.position == 1 ? .title : .title2)
                                        .foregroundStyle(result.position == 1 ? JoyPalette.sunflower : .secondary)
                                        .symbolEffect(.bounce, value: detail.session?.sessionKey)
                                    Text(result.nameAcronym ?? result.name)
                                        .font(.headline.weight(.black))
                                    Text(result.teamName ?? "")
                                        .font(.caption2)
                                        .foregroundStyle(.secondary)
                                        .lineLimit(1)
                                    Text("P\(result.position)")
                                        .font(.caption.weight(.black))
                                        .foregroundStyle(JoyPalette.coral)
                                }
                                .frame(maxWidth: .infinity)
                                .padding(.vertical, result.position == 1 ? 18 : 13)
                                .background(.background, in: .rect(cornerRadius: 18, style: .continuous))
                                .overlay { RoundedRectangle(cornerRadius: 18).stroke(.primary.opacity(0.07)) }
                            }
                        }
                        .joyfulListRow()
                    }
                }
                Section("Results") {
                    ForEach(detail.results) { result in
                        HStack {
                            Text(result.position > 0 ? String(result.position) : "–").frame(width: 24).foregroundStyle(.secondary)
                            VStack(alignment: .leading) {
                                Text(result.name).bold()
                                Text(result.teamName ?? "").font(.caption).foregroundStyle(.secondary)
                            }
                            Spacer()
                            Text(result.dnf ? "DNF" : (result.gapToLeader ?? "")).monospacedDigit()
                        }
                    }
                }
                Section("Race control") {
                    ForEach(detail.events.filter(\.significant)) { event in
                        VStack(alignment: .leading, spacing: 3) {
                            Text([event.flag, event.category].compactMap { $0 }.joined(separator: " · ")).font(.caption.bold()).foregroundStyle(.secondary)
                            Text(event.message)
                        }
                    }
                }
            }
        }
        .listStyle(.plain)
        .scrollContentBackground(.hidden)
        .background { JoyfulBackdrop() }
        .navigationTitle(race.name)
        .navigationBarTitleDisplayMode(.inline)
        .refreshable { await load() }
        .task(id: session.refreshGeneration) { await load() }
        .onDisappear { Task { try? await session.api.unwatchF1Race(id: race.sessionKey) } }
    }

    private func load() async {
        do { detail = try await session.api.watchF1Race(id: race.sessionKey) } catch { session.report(error) }
    }
}

private struct F1StandingsView: View {
    @Environment(SessionStore.self) private var session
    let year: Int
    @State private var standings: F1Standings?

    init(year: Int, initial: F1Standings? = nil) {
        self.year = year
        _standings = State(initialValue: initial)
    }

    var body: some View {
        List {
            Section("WDC · Drivers") {
                ForEach(standings?.drivers ?? []) { driver in
                    HStack {
                        Text(String(driver.position))
                            .font(.headline.monospacedDigit())
                            .foregroundStyle(driver.position <= 3 ? JoyPalette.coral : .secondary)
                            .frame(width: 28)
                        VStack(alignment: .leading) {
                            Text(driver.name).bold()
                            Text(driver.teamName ?? "").font(.caption).foregroundStyle(.secondary)
                        }
                        Spacer(); Text(driver.points.formatted()).monospacedDigit()
                    }
                }
            }
            Section("WCC · Constructors") {
                ForEach(standings?.constructors ?? []) { team in
                    HStack { Text(String(team.position)).foregroundStyle(.secondary); Text(team.teamName); Spacer(); Text(team.points.formatted()).monospacedDigit() }
                }
            }
        }
        .listStyle(.plain)
        .scrollContentBackground(.hidden)
        .background { JoyfulBackdrop() }
        .navigationTitle("\(year) Championship")
        .navigationBarTitleDisplayMode(.inline)
        .task { do { standings = try await session.api.f1Standings(year: year) } catch { session.report(error) } }
    }
}
