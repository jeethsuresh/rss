import SwiftUI

enum JoyPalette {
    static let coral = Color(red: 1.00, green: 0.30, blue: 0.33)
    static let violet = Color(red: 0.50, green: 0.20, blue: 0.98)
    static let sunflower = Color(red: 1.00, green: 0.68, blue: 0.04)
    static let cyan = Color(red: 0.00, green: 0.78, blue: 0.92)
    static let mint = Color(red: 0.10, green: 0.78, blue: 0.55)
    static let ink = Color(red: 0.04, green: 0.06, blue: 0.16)

    static let primary = LinearGradient(
        colors: [coral, violet],
        startPoint: .topLeading,
        endPoint: .bottomTrailing
    )

    static let sports = LinearGradient(
        colors: [violet, Color(red: 0.13, green: 0.29, blue: 0.95), cyan],
        startPoint: .topLeading,
        endPoint: .bottomTrailing
    )

    static let saved = LinearGradient(
        colors: [sunflower, coral],
        startPoint: .topLeading,
        endPoint: .bottomTrailing
    )

    static func gradient(for seed: String) -> LinearGradient {
        let value = seed.unicodeScalars.reduce(0) { ($0 &* 31) &+ Int($1.value) }
        let gradients: [[Color]] = [
            [coral, violet],
            [violet, cyan],
            [sunflower, coral],
            [mint, cyan],
            [Color.pink, violet]
        ]
        let index = Int(value.magnitude % UInt(gradients.count))
        let colors = gradients[index]
        return LinearGradient(colors: colors, startPoint: .topLeading, endPoint: .bottomTrailing)
    }
}

struct JoyfulBackdrop: View {
    var body: some View {
        ZStack {
            Color(.systemGroupedBackground)
            Circle()
                .fill(JoyPalette.violet.opacity(0.10))
                .frame(width: 360)
                .blur(radius: 80)
                .offset(x: 180, y: -280)
            Circle()
                .fill(JoyPalette.sunflower.opacity(0.08))
                .frame(width: 300)
                .blur(radius: 75)
                .offset(x: -180, y: 360)
        }
        .ignoresSafeArea()
    }
}

struct JoyHero: View {
    let eyebrow: String
    let title: String
    let subtitle: String
    let symbol: String
    var gradient: LinearGradient = JoyPalette.primary

    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @State private var floating = false

    var body: some View {
        ZStack(alignment: .bottomLeading) {
            RoundedRectangle(cornerRadius: 28, style: .continuous)
                .fill(gradient)

            Circle()
                .fill(.white.opacity(0.18))
                .frame(width: 150, height: 150)
                .offset(x: floating ? 245 : 220, y: floating ? -42 : -15)
                .blur(radius: 2)
            Circle()
                .fill(.white.opacity(0.12))
                .frame(width: 74, height: 74)
                .offset(x: floating ? 178 : 202, y: floating ? 22 : 6)

            VStack(alignment: .leading, spacing: 9) {
                Label(eyebrow.uppercased(), systemImage: symbol)
                    .font(.caption.weight(.black))
                    .tracking(1.4)
                Text(title)
                    .font(.system(.largeTitle, design: .rounded, weight: .black))
                    .lineLimit(2)
                Text(subtitle)
                    .font(.subheadline.weight(.medium))
                    .foregroundStyle(.white.opacity(0.85))
                    .lineLimit(2)
            }
            .foregroundStyle(.white)
            .padding(22)
        }
        .frame(minHeight: 170)
        .shadow(color: JoyPalette.violet.opacity(0.22), radius: 22, y: 12)
        .onAppear {
            guard !reduceMotion else { return }
            withAnimation(.easeInOut(duration: 3.2).repeatForever(autoreverses: true)) {
                floating = true
            }
        }
        .accessibilityElement(children: .combine)
    }
}

struct FeedIconView: View {
    let url: URL?
    let title: String
    var size: CGFloat = 28

    var body: some View {
        AsyncImage(url: url) { phase in
            switch phase {
            case let .success(image):
                image.resizable().scaledToFill()
            default:
                ZStack {
                    JoyPalette.gradient(for: title)
                    Text(title.prefix(1).uppercased())
                        .font(.system(size: size * 0.42, weight: .black, design: .rounded))
                        .foregroundStyle(.white)
                }
            }
        }
        .frame(width: size, height: size)
        .clipShape(.rect(cornerRadius: size * 0.28))
        .overlay {
            RoundedRectangle(cornerRadius: size * 0.28)
                .stroke(.white.opacity(0.35), lineWidth: 1)
        }
    }
}

struct TeamLogoView: View {
    let team: MlbTeam
    var size: CGFloat = 44
    @State private var image: UIImage?

    var body: some View {
        Group {
            if let image {
                Image(uiImage: image)
                    .resizable()
                    .scaledToFit()
                    .padding(size * 0.06)
            } else {
                Text(team.abbreviation.prefix(2))
                    .font(.system(size: size * 0.25, weight: .black, design: .rounded))
                    .foregroundStyle(.secondary)
            }
        }
        .frame(width: size, height: size)
        .background(Color.white, in: .circle)
        .clipShape(.circle)
        .overlay { Circle().stroke(.black.opacity(0.08), lineWidth: 1) }
        .accessibilityLabel(team.name)
        .task(id: team.logoUrl) {
            guard let logoURL = team.logoUrl, let url = URL(string: logoURL) else { return }
            image = await TeamLogoImageStore.shared.image(for: url)
        }
    }
}

struct CountryFlagView: View {
    let code: String?
    let countryName: String
    var size: CGFloat = 38

    var body: some View {
        Text(Self.flag(code: code, countryName: countryName))
            .font(.system(size: size))
            .frame(width: size * 1.35, height: size)
            .accessibilityLabel(countryName)
    }

    static func flag(code: String?, countryName: String) -> String {
        let normalized = code?.trimmingCharacters(in: .whitespacesAndNewlines).uppercased() ?? ""
        let alpha2 = alpha2Codes[normalized] ?? (normalized.count == 2 ? normalized : countryNames[countryName.lowercased()])
        guard let alpha2, alpha2.count == 2 else { return "🏁" }
        return alpha2.unicodeScalars.compactMap { scalar in
            UnicodeScalar(127_397 + scalar.value).map(String.init)
        }.joined()
    }

    private static let alpha2Codes = [
        "AUS": "AU", "AUT": "AT", "AZE": "AZ", "BEL": "BE", "BHR": "BH", "BRN": "BH",
        "BRA": "BR", "CAN": "CA", "CHN": "CN", "DEU": "DE", "ESP": "ES", "FRA": "FR",
        "GBR": "GB", "HUN": "HU", "ITA": "IT", "JPN": "JP", "KSA": "SA", "MCO": "MC",
        "MEX": "MX", "MON": "MC", "NED": "NL", "NLD": "NL", "POR": "PT", "QAT": "QA",
        "SAU": "SA", "SGP": "SG", "TUR": "TR", "UAE": "AE", "ARE": "AE", "USA": "US"
    ]

    private static let countryNames = [
        "australia": "AU", "austria": "AT", "azerbaijan": "AZ", "bahrain": "BH", "belgium": "BE",
        "brazil": "BR", "canada": "CA", "china": "CN", "france": "FR", "germany": "DE",
        "hungary": "HU", "italy": "IT", "japan": "JP", "mexico": "MX", "monaco": "MC",
        "netherlands": "NL", "portugal": "PT", "qatar": "QA", "saudi arabia": "SA", "singapore": "SG",
        "spain": "ES", "united arab emirates": "AE", "united kingdom": "GB", "united states": "US"
    ]
}

struct ArticleArtwork: View {
    let article: Article
    var width: CGFloat = 104
    var height: CGFloat = 104

    var body: some View {
        AsyncImage(url: article.leadImageURL) { phase in
            switch phase {
            case let .success(image):
                image.resizable().scaledToFill()
            default:
                ZStack {
                    JoyPalette.gradient(for: article.feedTitle ?? article.title)
                    Image(systemName: article.isReadLater ? "bookmark.fill" : "newspaper.fill")
                        .font(.system(size: min(width, height) * 0.28, weight: .bold))
                        .foregroundStyle(.white.opacity(0.92))
                    Circle()
                        .fill(.white.opacity(0.14))
                        .frame(width: width * 0.7)
                        .offset(x: width * 0.36, y: -height * 0.34)
                }
            }
        }
        .frame(width: width, height: height)
        .clipShape(.rect(cornerRadius: 18))
        .clipped()
    }
}

struct StatusPill: View {
    let text: String
    var color: Color = JoyPalette.violet

    var body: some View {
        Text(text.uppercased())
            .font(.caption2.weight(.black))
            .tracking(0.8)
            .foregroundStyle(color)
            .padding(.horizontal, 9)
            .padding(.vertical, 5)
            .background(color.opacity(0.12), in: .capsule)
    }
}

struct JoyPressStyle: ButtonStyle {
    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .scaleEffect(configuration.isPressed ? 0.975 : 1)
            .brightness(configuration.isPressed ? 0.04 : 0)
            .animation(.spring(response: 0.28, dampingFraction: 0.66), value: configuration.isPressed)
    }
}

extension View {
    func joyfulListRow() -> some View {
        listRowBackground(Color.clear)
            .listRowSeparator(.hidden)
            .listRowInsets(.init(top: 7, leading: 16, bottom: 7, trailing: 16))
    }
}
