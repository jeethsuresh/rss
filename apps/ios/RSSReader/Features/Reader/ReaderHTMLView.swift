import SwiftUI
import WebKit

struct ReaderHTMLView: UIViewRepresentable {
    let html: String
    let baseURL: URL?

    final class Coordinator {
        var lastDocument = ""
    }

    func makeCoordinator() -> Coordinator { Coordinator() }

    func makeUIView(context: Context) -> WKWebView {
        let configuration = WKWebViewConfiguration()
        configuration.websiteDataStore = .nonPersistent()
        configuration.defaultWebpagePreferences.allowsContentJavaScript = false
        let view = WKWebView(frame: .zero, configuration: configuration)
        view.isOpaque = false
        view.backgroundColor = .clear
        view.scrollView.isScrollEnabled = true
        view.scrollView.alwaysBounceVertical = true
        view.scrollView.contentInsetAdjustmentBehavior = .automatic
        return view
    }

    func updateUIView(_ view: WKWebView, context: Context) {
        let color = UIColor.label.resolvedColor(with: view.traitCollection).hexRGB
        let secondary = UIColor.secondaryLabel.resolvedColor(with: view.traitCollection).hexRGB
        let document = """
        <!doctype html><html><head><meta name="viewport" content="width=device-width,initial-scale=1">
        <style>
        :root { color-scheme: light dark; } body { font: -apple-system-body; color: \(color); background: transparent;
        line-height: 1.62; margin: 18px 18px 80px; overflow-wrap: anywhere; } img, video, iframe { max-width:100%; height:auto; border-radius:12px; }
        pre { overflow-x:auto; } blockquote { color:\(secondary); border-left:3px solid \(secondary); margin-left:0; padding-left:14px; }
        a { color:-apple-system-blue; } figure { margin:16px 0; } table { display:block; overflow-x:auto; }
        </style></head><body>\(html)</body></html>
        """
        guard context.coordinator.lastDocument != document else { return }
        context.coordinator.lastDocument = document
        view.loadHTMLString(document, baseURL: baseURL)
    }
}

private extension UIColor {
    var hexRGB: String {
        guard let components = cgColor.components else { return "#000000" }
        let values = components.count >= 3 ? components : [components[0], components[0], components[0]]
        return String(format: "#%02X%02X%02X", Int(values[0] * 255), Int(values[1] * 255), Int(values[2] * 255))
    }
}
