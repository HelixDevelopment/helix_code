// find_window_id.swift — HXC-108 video-QA harness window-id discovery (§11.4.154 window-scoped capture).
//
// Enumerates on-screen windows via CGWindowListCopyWindowInfo (Quartz) and prints the
// CGWindowID of the FIRST on-screen window whose owner-app name AND/OR window title
// contains the given (case-insensitive) substrings. Avoids the `System Events` AppleEvent
// path (which times out / is permission-gated on this host) — uses the public window list only.
//
// Usage:
//   swift find_window_id.swift --owner "Terminal" --title "HELIXCODE_OCR_PROBE"
//   swift find_window_id.swift --owner "helix-desktop"
//   swift find_window_id.swift --list                       # dump all on-screen windows (debug)
//
// Exit codes: 0 = printed a window id (stdout = bare integer id) ; 2 = no match ; 3 = bad args.
// §11.4.6: prints ONLY a real discovered id; on no-match exits 2 (honest, never a fabricated id).

import Foundation
import CoreGraphics

func argValue(_ flag: String) -> String? {
    let a = CommandLine.arguments
    if let i = a.firstIndex(of: flag), i + 1 < a.count { return a[i + 1] }
    return nil
}
let wantList = CommandLine.arguments.contains("--list")
let ownerNeedle = argValue("--owner")?.lowercased()
let titleNeedle = argValue("--title")?.lowercased()

if !wantList && ownerNeedle == nil && titleNeedle == nil {
    FileHandle.standardError.write("usage: find_window_id.swift [--owner <substr>] [--title <substr>] | --list\n".data(using: .utf8)!)
    exit(3)
}

// kCGWindowListOptionOnScreenOnly + exclude desktop elements.
let opts: CGWindowListOption = [.optionOnScreenOnly, .excludeDesktopElements]
guard let infoList = CGWindowListCopyWindowInfo(opts, kCGNullWindowID) as? [[String: Any]] else {
    FileHandle.standardError.write("CGWindowListCopyWindowInfo returned nil\n".data(using: .utf8)!)
    exit(2)
}

func field(_ d: [String: Any], _ k: String) -> String {
    return (d[k] as? String) ?? ""
}

if wantList {
    for d in infoList {
        let id = (d[kCGWindowNumber as String] as? Int) ?? -1
        let owner = field(d, kCGWindowOwnerName as String)
        let title = field(d, kCGWindowName as String)
        let layer = (d[kCGWindowLayer as String] as? Int) ?? -1
        print("id=\(id)\tlayer=\(layer)\towner=\(owner)\ttitle=\(title)")
    }
    exit(0)
}

for d in infoList {
    // Only normal-layer windows (layer 0) are real app windows; skip menubar/overlays.
    let layer = (d[kCGWindowLayer as String] as? Int) ?? -1
    if layer != 0 { continue }
    let owner = field(d, kCGWindowOwnerName as String).lowercased()
    let title = field(d, kCGWindowName as String).lowercased()
    let ownerOK = ownerNeedle == nil || owner.contains(ownerNeedle!)
    let titleOK = titleNeedle == nil || title.contains(titleNeedle!)
    if ownerOK && titleOK {
        if let id = d[kCGWindowNumber as String] as? Int {
            print(id)
            exit(0)
        }
    }
}
exit(2)
