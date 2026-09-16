import SwiftUI

struct LoadingOverlay: View {
    let title: String

    var body: some View {
        VStack(spacing: 18) {
            ZStack {
                Circle().fill(JoyPalette.violet.opacity(0.12)).frame(width: 76, height: 76)
                ProgressView().controlSize(.large).tint(JoyPalette.violet)
            }
            Text(title)
                .font(.headline)
                .foregroundStyle(.secondary)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .background { JoyfulBackdrop() }
    }
}

struct ArticleRow: View {
    let article: Article
    var feedIconURL: URL?

    var body: some View {
        HStack(alignment: .top, spacing: 14) {
            VStack(alignment: .leading, spacing: 10) {
                HStack(spacing: 8) {
                    FeedIconView(
                        url: feedIconURL,
                        title: article.feedTitle ?? "Story",
                        size: 25
                    )
                    Text(article.feedTitle?.uppercased() ?? "ARTICLE")
                        .font(.caption2.weight(.black))
                        .tracking(0.7)
                        .foregroundStyle(JoyPalette.violet)
                        .lineLimit(1)
                    Spacer(minLength: 4)
                    if let date = article.publishedAt?.serverDate {
                        Text(date, style: .relative)
                            .font(.caption2.weight(.semibold))
                            .foregroundStyle(.secondary)
                    }
                }

                HStack(alignment: .firstTextBaseline, spacing: 7) {
                    if !article.isRead {
                        Circle()
                            .fill(JoyPalette.coral)
                            .frame(width: 8, height: 8)
                            .shadow(color: JoyPalette.coral.opacity(0.5), radius: 4)
                    }
                    Text(article.title.isEmpty ? article.url : article.title)
                        .font(.system(.headline, design: .rounded, weight: article.isRead ? .semibold : .bold))
                        .foregroundStyle(article.isRead ? .secondary : .primary)
                        .lineLimit(3)
                }

                if !article.summary.isEmpty {
                    Text(article.summary)
                        .font(.subheadline)
                        .foregroundStyle(.secondary)
                        .lineLimit(2)
                }

                HStack(spacing: 7) {
                    if article.isStarred {
                        Label("Starred", systemImage: "star.fill")
                            .foregroundStyle(JoyPalette.sunflower)
                    }
                    if article.isReadLater {
                        Label("Read Later", systemImage: "bookmark.fill")
                            .foregroundStyle(JoyPalette.coral)
                    }
                    if article.priority != .none {
                        StatusPill(text: article.priority.rawValue, color: JoyPalette.violet)
                    }
                }
                .font(.caption2.weight(.bold))
                .labelStyle(.iconOnly)
            }
            .frame(maxWidth: .infinity, alignment: .leading)

            ArticleArtwork(article: article, width: 98, height: 112)
        }
        .padding(14)
        .background(.background.opacity(0.94), in: .rect(cornerRadius: 22, style: .continuous))
        .overlay {
            RoundedRectangle(cornerRadius: 22, style: .continuous)
                .stroke(.primary.opacity(0.06), lineWidth: 1)
        }
        .shadow(color: JoyPalette.ink.opacity(0.07), radius: 12, y: 7)
        .contentShape(.rect)
        .accessibilityElement(children: .combine)
    }
}

struct ScoreRow: View {
    let game: MlbGame

    private var isLive: Bool {
        let status = game.status.lowercased()
        return status.contains("live") || status.contains("progress")
    }

    var body: some View {
        VStack(spacing: 13) {
            HStack {
                HStack(spacing: 6) {
                    if isLive {
                        Image(systemName: "dot.radiowaves.left.and.right")
                            .foregroundStyle(JoyPalette.coral)
                            .symbolEffect(.variableColor.iterative, options: .repeating)
                    }
                    StatusPill(
                        text: game.statusDetail ?? game.status.replacingOccurrences(of: "_", with: " "),
                        color: isLive ? JoyPalette.coral : JoyPalette.violet
                    )
                }
                Spacer()
                if let date = game.gameDate.serverDate {
                    Text(date, format: .dateTime.month(.abbreviated).day().hour().minute())
                        .font(.caption.weight(.semibold))
                        .foregroundStyle(.secondary)
                }
            }

            HStack(spacing: 12) {
                team(game.awayTeam, score: game.awayScore)
                Text("@")
                    .font(.caption.weight(.black))
                    .foregroundStyle(.tertiary)
                team(game.homeTeam, score: game.homeScore)
            }
        }
        .padding(16)
        .background {
            RoundedRectangle(cornerRadius: 24, style: .continuous)
                .fill(.background.opacity(0.95))
                .overlay(alignment: .topTrailing) {
                    Circle()
                        .fill(JoyPalette.gradient(for: game.homeTeam.abbreviation))
                        .frame(width: 105, height: 105)
                        .blur(radius: 36)
                        .opacity(0.16)
                        .offset(x: 30, y: -42)
                }
        }
        .clipShape(.rect(cornerRadius: 24, style: .continuous))
        .overlay {
            RoundedRectangle(cornerRadius: 24, style: .continuous)
                .stroke(.primary.opacity(0.06), lineWidth: 1)
        }
        .shadow(color: JoyPalette.ink.opacity(0.07), radius: 12, y: 7)
        .accessibilityElement(children: .combine)
    }

    private func team(_ team: MlbTeam, score: Int?) -> some View {
        VStack(spacing: 7) {
            TeamLogoView(team: team, size: 50)
            Text(team.abbreviation)
                .font(.caption.weight(.black))
                .foregroundStyle(.secondary)
            Text(score.map(String.init) ?? "–")
                .font(.system(.title, design: .rounded, weight: .black))
                .contentTransition(.numericText())
        }
        .frame(maxWidth: .infinity)
    }
}
