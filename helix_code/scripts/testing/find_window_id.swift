// find_window_id.swift — HXC-112 helper.
//
// Prints the on-screen CGWindowID + bounds of the first window whose owner
// process name OR window title contains the substring given as $1
// (case-insensitive). Output (stdout, one line):
//
//   id=<CGWindowID> x=<int> y=<int> w=<int> h=<int> owner=<str> name=<str>
//
// Exit codes: 0 = found, 3 = not found, 2 = usage error.
//
// WHY a Swift helper and not osascript/CLI: there is no stock CLI that returns
// a CGWindowID; CGWindowListCopyWindowInfo is the documented API
// (https://developer.apple.com/documentation/coregraphics/cgwindowlistcopywindowinfo(_:_:)).
// kCGWindowNumber is the window id, kCGWindowOwnerName the app, kCGWindowBounds
// the on-screen rect used to position cliclick and to crop a window-scoped
// recording. (kCGWindowName / title needs Screen-Recording permission; we match
// on owner name primarily and fall back to title when available.)
//
// Run: swift find_window_id.swift "<match-substring>"
import CoreGraphics
import Foundation

guard CommandLine.arguments.count >= 2 else {
    FileHandle.standardError.write("usage: swift find_window_id.swift <match-substring>\n".data(using: .utf8)!)
    exit(2)
}
let needle = CommandLine.arguments[1].lowercased()

let opts = CGWindowListOption(arrayLiteral: .optionOnScreenOnly, .excludeDesktopElements)
guard let infos = CGWindowListCopyWindowInfo(opts, kCGNullWindowID) as? [[String: Any]] else {
    exit(3)
}

for w in infos {
    let owner = (w[kCGWindowOwnerName as String] as? String) ?? ""
    let name = (w[kCGWindowName as String] as? String) ?? ""
    let num = (w[kCGWindowNumber as String] as? Int) ?? -1
    // A real on-screen app window has layer 0; skip menu/overlay layers.
    let layer = (w[kCGWindowLayer as String] as? Int) ?? -1
    if layer != 0 { continue }
    if owner.lowercased().contains(needle) || name.lowercased().contains(needle) {
        var x = 0, y = 0, ww = 0, hh = 0
        if let b = w[kCGWindowBounds as String] as? [String: Any] {
            x = Int((b["X"] as? Double) ?? 0)
            y = Int((b["Y"] as? Double) ?? 0)
            ww = Int((b["Width"] as? Double) ?? 0)
            hh = Int((b["Height"] as? Double) ?? 0)
        }
        print("id=\(num) x=\(x) y=\(y) w=\(ww) h=\(hh) owner=\(owner) name=\(name)")
        exit(0)
    }
}
exit(3)
