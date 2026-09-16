import SwiftUI

struct SignInView: View {
    @Environment(SessionStore.self) private var session
    @AppStorage("lastServerURL") private var server = ""
    @State private var username = ""
    @State private var password = ""
    @State private var registering = false
    @State private var busy = false
    @State private var errorMessage: String?

    var body: some View {
        ZStack {
            JoyfulBackdrop()
            ScrollView {
                VStack(spacing: 24) {
                    Image("BrandMark")
                        .resizable()
                        .scaledToFit()
                        .frame(width: 132, height: 132)
                        .clipShape(.rect(cornerRadius: 30, style: .continuous))
                        .shadow(color: JoyPalette.violet.opacity(0.32), radius: 24, y: 14)

                    VStack(spacing: 8) {
                        Text(registering ? "Join the good news" : "Welcome back")
                            .font(.system(.largeTitle, design: .rounded, weight: .black))
                        Text("Stories worth your time. Sports worth cheering for.")
                            .font(.headline)
                            .foregroundStyle(.secondary)
                            .multilineTextAlignment(.center)
                    }

                    VStack(spacing: 16) {
                        VStack(spacing: 12) {
                            Label {
                                TextField("Server URL", text: $server)
                                    .textContentType(.URL)
                                    .keyboardType(.URL)
                                    .textInputAutocapitalization(.never)
                                    .autocorrectionDisabled()
                            } icon: { Image(systemName: "server.rack").foregroundStyle(JoyPalette.cyan) }
                            Label {
                                TextField("Username", text: $username)
                                    .textContentType(.username)
                                    .textInputAutocapitalization(.never)
                                    .autocorrectionDisabled()
                            } icon: { Image(systemName: "person.fill").foregroundStyle(JoyPalette.violet) }
                            Label {
                                SecureField("Password", text: $password)
                                    .textContentType(registering ? .newPassword : .password)
                            } icon: { Image(systemName: "lock.fill").foregroundStyle(JoyPalette.coral) }
                        }
                        .padding(16)
                        .background(.thinMaterial, in: .rect(cornerRadius: 22, style: .continuous))

                        if let errorMessage {
                            Label(errorMessage, systemImage: "exclamationmark.circle.fill")
                                .foregroundStyle(.red)
                                .font(.callout)
                                .frame(maxWidth: .infinity, alignment: .leading)
                        }

                        Button {
                            Task { await submit() }
                        } label: {
                            HStack {
                                if busy { ProgressView().tint(.white) }
                                Text(registering ? "Create my account" : "Let’s read")
                                    .font(.headline)
                                if !busy { Image(systemName: "arrow.right") }
                            }
                            .foregroundStyle(.white)
                            .frame(maxWidth: .infinity, minHeight: 54)
                            .background(JoyPalette.primary, in: .rect(cornerRadius: 18, style: .continuous))
                        }
                        .buttonStyle(JoyPressStyle())
                        .disabled(busy || server.isEmpty || username.isEmpty || password.count < 10)
                        .opacity(busy || server.isEmpty || username.isEmpty || password.count < 10 ? 0.55 : 1)

                        Button(registering ? "Already have an account? Sign in" : "New here? Create an account") {
                            withAnimation(.spring(response: 0.36, dampingFraction: 0.76)) {
                                registering.toggle()
                                errorMessage = nil
                            }
                        }
                        .font(.callout.weight(.semibold))
                    }
                    .padding(22)
                    .background(.background.opacity(0.82), in: .rect(cornerRadius: 30, style: .continuous))
                    .overlay {
                        RoundedRectangle(cornerRadius: 30, style: .continuous)
                            .stroke(.white.opacity(0.18), lineWidth: 1)
                    }
                    .shadow(color: JoyPalette.ink.opacity(0.10), radius: 24, y: 14)

                    Label("Use HTTPS outside your trusted local network", systemImage: "lock.shield.fill")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
                .padding(28)
                .frame(maxWidth: 560)
                .frame(maxWidth: .infinity)
            }
        }
        .tint(JoyPalette.violet)
    }

    private func submit() async {
        busy = true
        errorMessage = nil
        defer { busy = false }
        do {
            try await session.authenticate(server: server, username: username, password: password, register: registering)
        } catch {
            errorMessage = error.localizedDescription
        }
    }
}
