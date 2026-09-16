import SwiftUI
import UIKit
import WebKit

@MainActor
final class TeamLogoImageStore {
    static let shared = TeamLogoImageStore()

    private let cache = NSCache<NSURL, UIImage>()
    private var inFlight: [URL: Task<UIImage?, Never>] = [:]

    private init() {
        cache.countLimit = 64
    }

    func image(for url: URL) async -> UIImage? {
        if let cached = cache.object(forKey: url as NSURL) {
            return cached
        }
        if let task = inFlight[url] {
            return await task.value
        }

        let task: Task<UIImage?, Never> = Task { @MainActor in
            do {
                let (data, response) = try await URLSession.shared.data(from: url)
                try Task.checkCancellation()
                let mimeType = (response as? HTTPURLResponse)?.mimeType ?? ""
                if mimeType == "image/svg+xml" || url.pathExtension.lowercased() == "svg" {
                    return try await SVGLogoRenderer.render(data: data, baseURL: url)
                }
                return UIImage(data: data)
            } catch {
                return nil
            }
        }
        inFlight[url] = task
        let image = await task.value
        inFlight[url] = nil
        if let image {
            cache.setObject(image, forKey: url as NSURL)
        }
        return image
    }
}

@MainActor
private enum SVGLogoRenderer {
    static func render(data: Data, baseURL: URL) async throws -> UIImage {
        let size = CGSize(width: 180, height: 180)
        let webView = WKWebView(frame: CGRect(origin: .zero, size: size))
        webView.isOpaque = false
        webView.backgroundColor = .clear
        webView.underPageBackgroundColor = .clear
        webView.scrollView.backgroundColor = .clear
        webView.scrollView.isScrollEnabled = false

        let navigation = SVGNavigationDelegate()
        webView.navigationDelegate = navigation
        try await navigation.load(data: data, baseURL: baseURL, in: webView)

        let configuration = WKSnapshotConfiguration()
        configuration.rect = CGRect(origin: .zero, size: size)
        return try await webView.takeSnapshot(configuration: configuration)
    }
}

@MainActor
private final class SVGNavigationDelegate: NSObject, WKNavigationDelegate {
    private var continuation: CheckedContinuation<Void, Error>?

    func load(data: Data, baseURL: URL, in webView: WKWebView) async throws {
        try await withCheckedThrowingContinuation { continuation in
            self.continuation = continuation
            webView.load(
                data,
                mimeType: "image/svg+xml",
                characterEncodingName: "UTF-8",
                baseURL: baseURL.deletingLastPathComponent()
            )
        }
    }

    func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) {
        continuation?.resume()
        continuation = nil
    }

    func webView(_ webView: WKWebView, didFail navigation: WKNavigation!, withError error: Error) {
        continuation?.resume(throwing: error)
        continuation = nil
    }

    func webView(_ webView: WKWebView, didFailProvisionalNavigation navigation: WKNavigation!, withError error: Error) {
        continuation?.resume(throwing: error)
        continuation = nil
    }
}
