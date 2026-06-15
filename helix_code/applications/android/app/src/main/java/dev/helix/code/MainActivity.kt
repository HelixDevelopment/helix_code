package dev.helix.code

import android.os.Bundle
import android.widget.Button
import android.widget.TextView
import androidx.appcompat.app.AppCompatActivity
import androidx.core.view.WindowCompat
import androidx.recyclerview.widget.LinearLayoutManager
import androidx.recyclerview.widget.RecyclerView
import org.json.JSONArray
import org.json.JSONObject

class MainActivity : AppCompatActivity() {

    private lateinit var connectionStatusText: TextView
    private lateinit var userText: TextView
    private lateinit var tasksRecyclerView: RecyclerView
    private lateinit var connectButton: Button

    private val tasks = mutableListOf<Task>()

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        // Opt out of forced edge-to-edge (SDK 35 default) so the status text and
        // Connect button are inset below the action bar and above the navigation
        // bar instead of being drawn underneath the system bars.
        WindowCompat.setDecorFitsSystemWindows(window, true)
        setContentView(R.layout.activity_main)

        // Initialize views
        connectionStatusText = findViewById(R.id.connectionStatusText)
        userText = findViewById(R.id.userText)
        tasksRecyclerView = findViewById(R.id.tasksRecyclerView)
        connectButton = findViewById(R.id.connectButton)

        // Setup RecyclerView
        tasksRecyclerView.layoutManager = LinearLayoutManager(this)
        tasksRecyclerView.adapter = TaskAdapter(tasks) { task ->
            showTaskDetails(task)
        }

        // Setup button click
        connectButton.setOnClickListener {
            toggleConnection()
        }

        // Initialize mobile core
        MobileCore.shared.initialize()

        // Update UI
        updateUI()
    }

    private fun toggleConnection() {
        if (MobileCore.shared.isConnected) {
            MobileCore.shared.disconnect()
        } else {
            // For demo purposes, connect with test credentials. localhost:8080
            // is tunneled to the host's running HelixCode server via
            // `adb reverse tcp:8080 tcp:8080`. A successful login yields a real
            // JWT used to fetch the live server task list.
            val connected = MobileCore.shared.connect(
                "http://localhost:8080",
                "testuser",
                "testpass123"
            )
            if (!connected) {
                showAlert("Connection Failed", "Could not connect to server")
            }
        }
        updateUI()
    }

    private fun updateUI() {
        // Update connection status
        val isConnected = MobileCore.shared.isConnected
        connectionStatusText.text = if (isConnected) "Connected" else "Disconnected"
        connectionStatusText.setTextColor(
            if (isConnected) android.graphics.Color.GREEN else android.graphics.Color.RED
        )

        // Update user
        userText.text = "User: ${MobileCore.shared.currentUser}"

        // Update tasks
        loadTasks()

        // Update button
        connectButton.text = if (isConnected) "Disconnect" else "Connect"
    }

    private fun loadTasks() {
        val tasksJson = MobileCore.shared.tasks
        tasks.clear()

        try {
            // The Go core returns an object: {"tasks":[...], "total":N, "source":"..."}.
            // Parse the wrapper object then read the "tasks" array. Use optional
            // getters with fallbacks so both client-created tasks (name/description/
            // progress) and server tasks (id/type/status) render without crashing.
            val root = JSONObject(tasksJson)
            val jsonArray = root.optJSONArray("tasks") ?: JSONArray()
            for (i in 0 until jsonArray.length()) {
                val taskObj = jsonArray.getJSONObject(i)
                val name = when {
                    taskObj.has("name") -> taskObj.optString("name")
                    taskObj.has("type") -> taskObj.optString("type")
                    else -> "Task"
                }
                val task = Task(
                    id = taskObj.optString("id"),
                    name = name,
                    description = taskObj.optString("description", ""),
                    status = taskObj.optString("status", "unknown"),
                    progress = taskObj.optInt("progress", 0)
                )
                tasks.add(task)
            }
        } catch (e: Exception) {
            e.printStackTrace()
        }

        tasksRecyclerView.adapter?.notifyDataSetChanged()
    }

    private fun showTaskDetails(task: Task) {
        val dialog = android.app.AlertDialog.Builder(this)
            .setTitle(task.name)
            .setMessage(task.description)
            .setPositiveButton("OK", null)
            .create()
        dialog.show()
    }

    private fun showAlert(title: String, message: String) {
        val dialog = android.app.AlertDialog.Builder(this)
            .setTitle(title)
            .setMessage(message)
            .setPositiveButton("OK", null)
            .create()
        dialog.show()
    }

    override fun onDestroy() {
        super.onDestroy()
        MobileCore.shared.close()
    }
}

data class Task(
    val id: String,
    val name: String,
    val description: String,
    val status: String,
    val progress: Int
)