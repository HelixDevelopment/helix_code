//
//  AppDelegate.swift
//  HelixCode
//
//  Created by HelixCode on 2025-11-02.
//
//  Sets up the window and root view controller programmatically so the app
//  launches deterministically on the simulator without depending on a
//  storyboard main-interface connection (the IBOutlets in ViewController are
//  optional; the controller builds its UI in code). Initializes the real Go
//  mobile core on launch.
//

import UIKit

@main
class AppDelegate: UIResponder, UIApplicationDelegate {

    var window: UIWindow?

    func application(_ application: UIApplication, didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]?) -> Bool {
        // Initialize the real Go mobile core (cgo into the Go runtime).
        MobileCore.shared.initialize()

        // Build the UI programmatically (storyboard-independent, crash-proof).
        let window = UIWindow(frame: UIScreen.main.bounds)
        let nav = UINavigationController(rootViewController: ViewController())
        window.rootViewController = nav
        window.makeKeyAndVisible()
        self.window = window

        return true
    }

    func applicationWillTerminate(_ application: UIApplication) {
        MobileCore.shared.close()
    }
}
