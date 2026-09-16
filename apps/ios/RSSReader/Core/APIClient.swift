import Foundation

enum APIClientError: LocalizedError, Sendable {
    case invalidServerURL
    case invalidResponse
    case unauthorized
    case server(status: Int, code: String?, message: String)

    var errorDescription: String? {
        switch self {
        case .invalidServerURL: "Enter a valid server URL."
        case .invalidResponse: "The server returned an unreadable response."
        case .unauthorized: "Your session has expired. Please sign in again."
        case let .server(_, _, message): message
        }
    }
}

private struct APIErrorEnvelope: Decodable, Sendable {
    struct Detail: Decodable, Sendable {
        let code: String?
        let message: String
    }
    let error: Detail
}

private struct RPCRequest<Params: Encodable & Sendable>: Encodable, Sendable {
    let method: String
    let params: Params
}

actor APIClient {
    private let baseURL: URL
    private let token: String?
    private let session: URLSession
    private let encoder: JSONEncoder
    private let decoder: JSONDecoder

    init(baseURL: URL, token: String? = nil, session: URLSession = .shared) {
        self.baseURL = baseURL
        self.token = token
        self.session = session
        self.encoder = JSONEncoder()
        self.decoder = JSONDecoder()
    }

    static func normalizedServerURL(_ input: String) throws -> URL {
        let trimmed = input.trimmingCharacters(in: .whitespacesAndNewlines)
        let candidate = trimmed.contains("://") ? trimmed : "https://\(trimmed)"
        guard var components = URLComponents(string: candidate),
              let scheme = components.scheme?.lowercased(),
              ["http", "https"].contains(scheme),
              components.host != nil else {
            throw APIClientError.invalidServerURL
        }
        components.path = components.path.trimmingCharacters(in: CharacterSet(charactersIn: "/"))
        guard let url = components.url else { throw APIClientError.invalidServerURL }
        return url
    }

    func authenticate(username: String, password: String, register: Bool) async throws -> AuthResult {
        struct Credentials: Encodable, Sendable { let username: String; let password: String }
        return try await request(
            register ? "/v1/auth/register" : "/v1/auth/login",
            method: "POST",
            body: Credentials(username: username, password: password),
            authenticated: false
        )
    }

    func configuration() async throws -> ServerConfiguration {
        try await request("/v1/web/config", authenticated: false)
    }

    func currentUser() async throws -> User {
        try await request("/v1/me")
    }

    func rpc<Response: Decodable & Sendable>(
        _ method: String,
        as response: Response.Type = Response.self
    ) async throws -> Response {
        try await rpc(method, params: EmptyRequest(), as: response)
    }

    func rpc<Params: Encodable & Sendable, Response: Decodable & Sendable>(
        _ method: String,
        params: Params,
        as response: Response.Type = Response.self
    ) async throws -> Response {
        try await request(
            "/v1/rpc",
            method: "POST",
            body: RPCRequest(method: method, params: params),
            extraHeaders: ["X-RSS-CSRF": "1"]
        )
    }

    func events() -> AsyncThrowingStream<BackendEvent, Error> {
        var request = URLRequest(url: baseURL.appending(path: "/v1/events"))
        request.setValue("text/event-stream", forHTTPHeaderField: "Accept")
        if let token { request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization") }
        let eventRequest = request
        let session = session
        let decoder = decoder

        return AsyncThrowingStream { continuation in
            let task = Task {
                do {
                    let (bytes, response) = try await session.bytes(for: eventRequest)
                    guard let http = response as? HTTPURLResponse else {
                        throw APIClientError.invalidResponse
                    }
                    guard http.statusCode == 200 else {
                        if http.statusCode == 401 { throw APIClientError.unauthorized }
                        throw APIClientError.server(status: http.statusCode, code: nil, message: "Event stream failed.")
                    }
                    for try await line in bytes.lines {
                        try Task.checkCancellation()
                        guard line.hasPrefix("data: ") else { continue }
                        let data = Data(line.dropFirst(6).utf8)
                        continuation.yield(try decoder.decode(BackendEvent.self, from: data))
                    }
                    continuation.finish()
                } catch is CancellationError {
                    continuation.finish()
                } catch {
                    continuation.finish(throwing: error)
                }
            }
            continuation.onTermination = { @Sendable _ in task.cancel() }
        }
    }

    private func request<Response: Decodable & Sendable>(
        _ path: String,
        method: String = "GET",
        authenticated: Bool = true
    ) async throws -> Response {
        try await request(path, method: method, body: Optional<Int>.none, authenticated: authenticated)
    }

    private func request<Body: Encodable & Sendable, Response: Decodable & Sendable>(
        _ path: String,
        method: String,
        body: Body?,
        authenticated: Bool = true,
        extraHeaders: [String: String] = [:]
    ) async throws -> Response {
        var request = URLRequest(url: baseURL.appending(path: path))
        request.httpMethod = method
        request.timeoutInterval = 30
        request.setValue("application/json", forHTTPHeaderField: "Accept")
        if authenticated {
            guard let token else { throw APIClientError.unauthorized }
            request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }
        for (key, value) in extraHeaders { request.setValue(value, forHTTPHeaderField: key) }
        if let body {
            request.httpBody = try encoder.encode(body)
            request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        }

        let (data, response) = try await session.data(for: request)
        guard let http = response as? HTTPURLResponse else { throw APIClientError.invalidResponse }
        guard (200..<300).contains(http.statusCode) else {
            if http.statusCode == 401 { throw APIClientError.unauthorized }
            let envelope = try? decoder.decode(APIErrorEnvelope.self, from: data)
            throw APIClientError.server(
                status: http.statusCode,
                code: envelope?.error.code,
                message: envelope?.error.message ?? "Server request failed (\(http.statusCode))."
            )
        }
        if Response.self == EmptyResponse.self, data.isEmpty {
            return EmptyResponse() as! Response
        }
        return try decoder.decode(Response.self, from: data)
    }
}
