import Foundation
import Observation

@MainActor
@Observable
final class SessionStore {
    enum Phase: Equatable {
        case restoring
        case signedOut
        case signedIn(User)
    }

    private(set) var phase: Phase = .restoring
    private(set) var serverURL: URL?
    private(set) var refreshGeneration = 0
    var alertMessage: String?

    private let vault: CredentialVault
    private var apiClient: APIClient?
    private var eventTask: Task<Void, Never>?

    init(vault: CredentialVault = CredentialVault()) {
        self.vault = vault
    }

    var api: APIClient {
        get throws {
            guard let apiClient else { throw APIClientError.unauthorized }
            return apiClient
        }
    }

    func restore() async {
        do {
            guard let stored = try await vault.load() else {
                phase = .signedOut
                return
            }
            let client = APIClient(baseURL: stored.serverURL, token: stored.token)
            let user = try await client.currentUser()
            activate(client: client, serverURL: stored.serverURL, user: user)
        } catch {
            try? await vault.clear()
            phase = .signedOut
        }
    }

    func authenticate(server: String, username: String, password: String, register: Bool) async throws {
        let url = try APIClient.normalizedServerURL(server)
        let unauthenticated = APIClient(baseURL: url)
        let result = try await unauthenticated.authenticate(username: username, password: password, register: register)
        let client = APIClient(baseURL: url, token: result.token)
        try await vault.save(StoredSession(serverURL: url, token: result.token, user: result.user))
        activate(client: client, serverURL: url, user: result.user)
    }

    func signOut() async {
        eventTask?.cancel()
        eventTask = nil
        apiClient = nil
        serverURL = nil
        phase = .signedOut
        try? await vault.clear()
    }

    func report(_ error: Error) {
        if error.isExpectedCancellation {
            return
        } else if case APIClientError.unauthorized = error {
            Task { await signOut() }
        } else {
            alertMessage = error.localizedDescription
        }
    }

    private func activate(client: APIClient, serverURL: URL, user: User) {
        apiClient = client
        self.serverURL = serverURL
        phase = .signedIn(user)
        eventTask?.cancel()
        eventTask = Task { [weak self] in
            while !Task.isCancelled {
                do {
                    for try await _ in await client.events() {
                        guard let self else { return }
                        self.refreshGeneration &+= 1
                    }
                } catch is CancellationError {
                    return
                } catch APIClientError.unauthorized {
                    guard let self else { return }
                    await self.signOut()
                    return
                } catch {
                    try? await Task.sleep(for: .seconds(3))
                }
            }
        }
    }
}

private extension Error {
    var isExpectedCancellation: Bool {
        if self is CancellationError {
            return true
        }

        return (self as? URLError)?.code == .cancelled
    }
}
