import Foundation

enum SportsPresentation {
    static func latestGames(
        from games: [MlbGame],
        trackedTeamIDs: Set<Int> = [],
        limit: Int = 12
    ) -> [MlbGame] {
        let scored = games.filter { $0.status == "live" || $0.status == "final" }
        let candidates = scored.isEmpty ? games : scored
        return Array(candidates.sorted {
            let leftTracked = trackedTeamIDs.contains($0.awayTeam.id) || trackedTeamIDs.contains($0.homeTeam.id)
            let rightTracked = trackedTeamIDs.contains($1.awayTeam.id) || trackedTeamIDs.contains($1.homeTeam.id)
            if leftTracked != rightTracked { return leftTracked }
            return ($0.gameDate.serverDate ?? .distantPast) > ($1.gameDate.serverDate ?? .distantPast)
        }.prefix(limit))
    }

    static func orderedRaces(_ races: [F1Race]) -> [F1Race] {
        races.sorted {
            ($0.dateStart.serverDate ?? .distantPast) < ($1.dateStart.serverDate ?? .distantPast)
        }
    }

    static func previousRace(in races: [F1Race], now: Date = Date()) -> F1Race? {
        let ordered = orderedRaces(races)
        return ordered.last { $0.status == "completed" }
            ?? ordered.last { ($0.dateStart.serverDate ?? .distantFuture) <= now }
    }

    static func nextRace(in races: [F1Race], now: Date = Date()) -> F1Race? {
        let ordered = orderedRaces(races)
        return ordered.first { $0.status == "in_progress" }
            ?? ordered.first { $0.status == "scheduled" && ($0.dateStart.serverDate ?? .distantPast) > now }
    }
}
