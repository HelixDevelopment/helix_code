//
//  HelixTheme.swift
//  HelixCode
//
//  HelixCode brand dark theme — derived from assets/Logo.png
//  (lime-green → teal nautilus spiral on a deep-green ground).
//
//  This is the single source of truth for the iOS client's brand palette.
//  The app is UIKit-based (programmatic UIViews, see ViewController.swift),
//  so the canonical palette is expressed as `UIColor` extensions. A SwiftUI
//  `Color` mirror is provided too (compiled only when SwiftUI is available)
//  so future SwiftUI surfaces inherit the exact same FACT palette.
//
//  FACT palette (hex → 0–255 RGB):
//    bgBase     #0E1310  ( 14,  19,  16)
//    bgSurface  #18201A  ( 24,  32,  26)
//    bgRaised   #202A22  ( 32,  42,  34)
//    border     #2A352C  ( 42,  53,  44)
//    primary    #A8DD22  (168, 221,  34)  lime
//    primaryDim #7FA81B  (127, 168,  27)
//    secondary  #8FC9B8  (143, 201, 184)  teal
//    fgText     #ECF3E8  (236, 243, 232)
//    fgMuted    #9DB0A0  (157, 176, 160)
//    error      #E06A5A  (224, 106,  90)
//

import UIKit

extension UIColor {
    /// Build a fully-opaque UIColor from 0–255 component values (DRY helper).
    fileprivate static func helix(_ r: CGFloat, _ g: CGFloat, _ b: CGFloat) -> UIColor {
        return UIColor(red: r / 255.0, green: g / 255.0, blue: b / 255.0, alpha: 1.0)
    }

    // Backgrounds / surfaces.
    static let helixBgBase    = UIColor.helix( 14,  19,  16) // #0E1310
    static let helixBgSurface = UIColor.helix( 24,  32,  26) // #18201A
    static let helixBgRaised  = UIColor.helix( 32,  42,  34) // #202A22
    static let helixBorder    = UIColor.helix( 42,  53,  44) // #2A352C

    // Brand accents.
    static let helixPrimary    = UIColor.helix(168, 221,  34) // #A8DD22 lime
    static let helixPrimaryDim  = UIColor.helix(127, 168,  27) // #7FA81B
    static let helixSecondary   = UIColor.helix(143, 201, 184) // #8FC9B8 teal

    // Foreground / text.
    static let helixFgText  = UIColor.helix(236, 243, 232) // #ECF3E8
    static let helixFgMuted = UIColor.helix(157, 176, 160) // #9DB0A0

    // State.
    static let helixError = UIColor.helix(224, 106,  90) // #E06A5A
}

#if canImport(SwiftUI)
import SwiftUI

@available(iOS 13.0, *)
extension Color {
    static let helixBgBase    = Color(red: 14 / 255,  green: 19 / 255,  blue: 16 / 255)  // #0E1310
    static let helixBgSurface = Color(red: 24 / 255,  green: 32 / 255,  blue: 26 / 255)  // #18201A
    static let helixBgRaised  = Color(red: 32 / 255,  green: 42 / 255,  blue: 34 / 255)  // #202A22
    static let helixBorder    = Color(red: 42 / 255,  green: 53 / 255,  blue: 44 / 255)  // #2A352C

    static let helixPrimary    = Color(red: 168 / 255, green: 221 / 255, blue: 34 / 255) // #A8DD22 lime
    static let helixPrimaryDim  = Color(red: 127 / 255, green: 168 / 255, blue: 27 / 255) // #7FA81B
    static let helixSecondary   = Color(red: 143 / 255, green: 201 / 255, blue: 184 / 255)// #8FC9B8 teal

    static let helixFgText  = Color(red: 236 / 255, green: 243 / 255, blue: 232 / 255)    // #ECF3E8
    static let helixFgMuted = Color(red: 157 / 255, green: 176 / 255, blue: 160 / 255)    // #9DB0A0

    static let helixError = Color(red: 224 / 255, green: 106 / 255, blue: 90 / 255)       // #E06A5A
}
#endif
