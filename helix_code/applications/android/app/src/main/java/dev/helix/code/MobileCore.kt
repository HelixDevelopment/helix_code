package dev.helix.code

import org.json.JSONObject
import dev.helix.code.core.core.Core
import dev.helix.code.core.core.MobileCore as GoMobileCore

/**
 * Kotlin bridge over the gomobile-produced binding
 * (dev.helix.code/shared/mobile_core, package `core`).
 *
 * The real Go core (mobilecore.aar) backs every call here — there is no
 * simulation. `Core.newMobileCore()` is the gobind static factory; the
 * returned [GoMobileCore] proxy dispatches over JNI into the Go runtime.
 */
object MobileCore {
    val shared = MobileCoreInstance()
}

class MobileCoreInstance {
    private var core: GoMobileCore? = null

    init {
        core = Core.newMobileCore()
    }

    fun initialize() {
        try {
            core?.initialize()
        } catch (e: Exception) {
            e.printStackTrace()
        }
    }

    fun connect(serverURL: String, username: String, password: String): Boolean {
        return try {
            core?.connect(serverURL, username, password)
            true
        } catch (e: Exception) {
            e.printStackTrace()
            false
        }
    }

    fun disconnect() {
        try {
            core?.disconnect()
        } catch (e: Exception) {
            e.printStackTrace()
        }
    }

    val isConnected: Boolean
        get() = core?.isConnected ?: false

    val currentUser: String
        get() = core?.currentUser ?: ""

    val dashboardData: String
        get() = core?.dashboardData ?: "{}"

    val tasks: String
        get() = core?.tasks ?: "[]"

    val workers: String
        get() = core?.workers ?: "[]"

    fun createTask(name: String, description: String): String {
        return core?.createTask(name, description) ?: "{\"error\": \"Core not initialized\"}"
    }

    fun generate(prompt: String): String {
        return try {
            core?.generate(prompt) ?: ""
        } catch (e: Exception) {
            e.printStackTrace()
            "{\"error\": \"${e.message}\"}"
        }
    }

    fun sendNotification(title: String, message: String, type: String): Boolean {
        return try {
            val result = core?.sendNotification(title, message, type)
            val json = JSONObject(result ?: "{}")
            json.optBoolean("success", false)
        } catch (e: Exception) {
            e.printStackTrace()
            false
        }
    }

    val theme: String
        get() = core?.theme ?: "{}"

    fun setTheme(themeName: String): Boolean {
        return core?.setTheme(themeName) ?: false
    }

    val availableThemes: String
        get() = core?.availableThemes ?: "[]"

    fun close() {
        try {
            core?.close()
        } catch (e: Exception) {
            e.printStackTrace()
        }
    }
}
