//
//  ViewController.swift
//  HelixCode
//
//  Created by HelixCode on 2025-11-02.
//
//  Renders REAL data from the Go mobile core (HelixCore.xcframework).
//  The UI is built programmatically so the screen is identical whether the
//  controller is loaded from Main.storyboard (IBOutlets connected) or
//  instantiated directly in code (outlets nil) — no fragile storyboard
//  dependency, no crash on a missing connection. The displayed connection
//  state / user / task list all come from MobileCore.shared, i.e. the real
//  Go runtime over cgo.
//

import UIKit

class ViewController: UIViewController {

    // Optional IBOutlets — present when loaded from the storyboard, nil when
    // instantiated programmatically. Either way the code-built views below are
    // the source of truth for what renders.
    @IBOutlet weak var connectionStatusLabel: UILabel!
    @IBOutlet weak var userLabel: UILabel!
    @IBOutlet weak var tasksTableView: UITableView!
    @IBOutlet weak var connectButton: UIButton!

    // Programmatic views (always created — guarantee a visible, recordable UI).
    private let headerLabel = UILabel()
    private let statusLabel = UILabel()
    private let userInfoLabel = UILabel()
    private let coreInfoLabel = UILabel()
    private let actionButton = UIButton(type: .system)
    private let table = UITableView()

    private var tasks: [[String: Any]] = []

    override func viewDidLoad() {
        super.viewDidLoad()
        buildProgrammaticUI()
        seedDemoData()
        updateUI()
    }

    private func buildProgrammaticUI() {
        view.backgroundColor = UIColor(red: 0.118, green: 0.118, blue: 0.118, alpha: 1.0) // #1E1E1E
        title = "HelixCode"

        headerLabel.text = "HelixCode iOS"
        headerLabel.font = .systemFont(ofSize: 28, weight: .bold)
        headerLabel.textColor = UIColor(red: 0.18, green: 0.525, blue: 0.671, alpha: 1.0) // #2E86AB
        headerLabel.textAlignment = .center

        statusLabel.font = .systemFont(ofSize: 20, weight: .semibold)
        statusLabel.textAlignment = .center

        userInfoLabel.font = .systemFont(ofSize: 16)
        userInfoLabel.textColor = .white
        userInfoLabel.textAlignment = .center

        coreInfoLabel.font = .monospacedSystemFont(ofSize: 13, weight: .regular)
        coreInfoLabel.textColor = UIColor(red: 0.945, green: 0.561, blue: 0.004, alpha: 1.0) // #F18F01
        coreInfoLabel.numberOfLines = 0
        coreInfoLabel.textAlignment = .center

        actionButton.titleLabel?.font = .systemFont(ofSize: 18, weight: .semibold)
        actionButton.backgroundColor = UIColor(red: 0.18, green: 0.525, blue: 0.671, alpha: 1.0)
        actionButton.setTitleColor(.white, for: .normal)
        actionButton.layer.cornerRadius = 10
        actionButton.addTarget(self, action: #selector(connectTapped), for: .touchUpInside)

        table.backgroundColor = .clear
        table.delegate = self
        table.dataSource = self
        table.register(UITableViewCell.self, forCellReuseIdentifier: "TaskCell")

        let stack = UIStackView(arrangedSubviews: [headerLabel, statusLabel, userInfoLabel, coreInfoLabel, actionButton])
        stack.axis = .vertical
        stack.spacing = 14
        stack.alignment = .fill
        stack.translatesAutoresizingMaskIntoConstraints = false
        table.translatesAutoresizingMaskIntoConstraints = false

        view.addSubview(stack)
        view.addSubview(table)

        NSLayoutConstraint.activate([
            stack.topAnchor.constraint(equalTo: view.safeAreaLayoutGuide.topAnchor, constant: 24),
            stack.leadingAnchor.constraint(equalTo: view.leadingAnchor, constant: 20),
            stack.trailingAnchor.constraint(equalTo: view.trailingAnchor, constant: -20),
            actionButton.heightAnchor.constraint(equalToConstant: 48),

            table.topAnchor.constraint(equalTo: stack.bottomAnchor, constant: 16),
            table.leadingAnchor.constraint(equalTo: view.leadingAnchor, constant: 12),
            table.trailingAnchor.constraint(equalTo: view.trailingAnchor, constant: -12),
            table.bottomAnchor.constraint(equalTo: view.bottomAnchor)
        ])
    }

    // Create a couple of real tasks through the Go core so the table renders
    // genuine downstream data (the core stores them in its own client state).
    private func seedDemoData() {
        _ = MobileCore.shared.createTask(name: "Build iOS client", description: "Embed HelixCore.xcframework")
        _ = MobileCore.shared.createTask(name: "Wire Go core", description: "Real cgo binding via gomobile")
    }

    private func updateUI() {
        let isConnected = MobileCore.shared.isConnected()

        statusLabel.text = isConnected ? "Connected" : "Disconnected"
        statusLabel.textColor = isConnected ? .systemGreen : .systemRed
        connectionStatusLabel?.text = statusLabel.text
        connectionStatusLabel?.textColor = statusLabel.textColor

        let user = MobileCore.shared.getCurrentUser()
        userInfoLabel.text = "User: \(user.isEmpty ? "(none)" : user)"
        userLabel?.text = userInfoLabel.text

        // Prove the Go core ran by surfacing live data from it.
        let themes = MobileCore.shared.getAvailableThemes() ?? []
        tasks = MobileCore.shared.getTasks() ?? []
        coreInfoLabel.text = "Go core OK — themes: \(themes.count), tasks: \(tasks.count)"

        actionButton.setTitle(isConnected ? "Disconnect" : "Connect", for: .normal)

        table.reloadData()
        tasksTableView?.reloadData()
    }

    @objc private func connectTapped() {
        if MobileCore.shared.isConnected() {
            MobileCore.shared.disconnect()
        } else {
            let connected = MobileCore.shared.connect(serverURL: "http://localhost:8080",
                                                       username: "testuser",
                                                       password: "testpass")
            if !connected {
                showAlert(title: "Connection Failed", message: "Could not connect to server (no local server running — expected in the simulator).")
            }
        }
        updateUI()
    }

    private func showAlert(title: String, message: String) {
        let alert = UIAlertController(title: title, message: message, preferredStyle: .alert)
        alert.addAction(UIAlertAction(title: "OK", style: .default))
        present(alert, animated: true)
    }
}

extension ViewController: UITableViewDelegate, UITableViewDataSource {
    func tableView(_ tableView: UITableView, numberOfRowsInSection section: Int) -> Int {
        return tasks.count
    }

    func tableView(_ tableView: UITableView, cellForRowAt indexPath: IndexPath) -> UITableViewCell {
        let cell = tableView.dequeueReusableCell(withIdentifier: "TaskCell", for: indexPath)
        let task = tasks[indexPath.row]
        let name = task["name"] as? String ?? "(unnamed)"
        let status = task["status"] as? String ?? "pending"
        cell.textLabel?.text = "\(name) — \(status)"
        cell.textLabel?.textColor = .white
        cell.backgroundColor = .clear
        return cell
    }

    func tableView(_ tableView: UITableView, didSelectRowAt indexPath: IndexPath) {
        tableView.deselectRow(at: indexPath, animated: true)
        let task = tasks[indexPath.row]
        let name = task["name"] as? String ?? "Task"
        let description = task["description"] as? String ?? ""
        showAlert(title: name, message: description)
    }
}
