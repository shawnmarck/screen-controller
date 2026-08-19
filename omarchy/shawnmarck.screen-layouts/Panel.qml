import QtQuick
import Quickshell
import Quickshell.Io
import qs.Commons
import qs.Ui

Panel {
  id: root
  moduleName: "shawnmarck.screen-layouts"
  ipcTarget: "shawnmarck.screen-layouts"

  readonly property string binary: setting("binary", "screen-controller")
  readonly property string tuiCommand: setting("tuiCommand", "omarchy-launch-tui screen-controller")
  readonly property string configPath: setting("config", "")
  readonly property int refreshMs: setting("refreshMs", 4000)
  readonly property string layoutIcon: "\uf0a07"

  property var profiles: []
  property string matchedId: ""
  property string hyprland: ""
  property string configFile: ""
  property string statusText: ""
  property string errorText: ""
  property bool busy: false
  property int selectedIndex: 0
  property bool cursorActive: false
  property string mode: "list"
  property string saveId: ""
  property string saveLabel: ""
  property string pendingDeleteId: ""

  readonly property var selected: {
    if (profiles.length === 0) return null
    return profiles[Math.max(0, Math.min(selectedIndex, profiles.length - 1))]
  }

  implicitWidth: button.implicitWidth
  implicitHeight: button.implicitHeight

  function argv(args) {
    var cmd = [root.binary]
    if (root.configPath !== "") cmd.push("--config", root.configPath)
    cmd.push("--json")
    return cmd.concat(args)
  }

  function parseState(raw) {
    var text = String(raw || "").trim()
    if (!text) return
    var data
    try {
      data = JSON.parse(text.split("\n").pop())
    } catch (e) {
      root.errorText = "Bad JSON from screen-controller"
      return
    }
    if (data.ok === false) {
      root.errorText = data.error || "screen-controller failed"
      return
    }
    root.errorText = ""
    root.profiles = data.profiles || []
    root.matchedId = data.matchedId || ""
    root.hyprland = data.hyprland || ""
    root.configFile = data.configPath || ""
    if (root.selectedIndex >= root.profiles.length)
      root.selectedIndex = Math.max(0, root.profiles.length - 1)
  }

  function refresh() {
    if (root.busy) return
    listProc.command = root.argv(["list"])
    listProc.running = true
  }

  function runAction(args, okMessage) {
    if (root.busy) return
    root.busy = true
    root.statusText = "Working..."
    actionProc.okMessage = okMessage || ""
    actionProc.command = root.argv(args)
    actionProc.running = true
  }

  function applySelected() {
    if (!root.selected) return
    runAction(["apply", root.selected.id], "Applied " + root.selected.label)
  }

  function saveCurrent() {
    var id = String(root.saveId || "").trim()
    var label = String(root.saveLabel || "").trim()
    if (id === "") {
      root.errorText = "Profile id is required"
      return
    }
    var args = ["save", id]
    if (label !== "") args = ["--label", label, "save", id]
    runAction(args, "Saved " + id)
    root.mode = "list"
  }

  function askDelete() {
    if (!root.selected) return
    root.pendingDeleteId = root.selected.id
    root.mode = "confirmDelete"
  }

  function confirmDelete() {
    if (root.pendingDeleteId === "") return
    runAction(["delete", root.pendingDeleteId], "Deleted " + root.pendingDeleteId)
    root.pendingDeleteId = ""
    root.mode = "list"
  }

  function relabelSelected() {
    if (!root.selected) return
    var label = String(root.saveLabel || "").trim()
    if (label === "") {
      root.errorText = "Label is required"
      return
    }
    runAction(["relabel", root.selected.id, label], "Updated label")
    root.mode = "list"
  }

  function openTui() {
    if (root.bar) root.bar.run(root.tuiCommand)
    root.close()
  }

  function moveCursor(dy) {
    if (root.mode !== "list" || root.profiles.length === 0) return
    root.cursorActive = true
    root.selectedIndex = Math.max(0, Math.min(root.profiles.length - 1, root.selectedIndex + dy))
  }

  onOpenedChanged: if (opened) {
    root.mode = "list"
    root.cursorActive = false
    root.saveId = ""
    root.saveLabel = root.selected ? root.selected.label : ""
    refresh()
    Qt.callLater(function() { if (keyCatcher) keyCatcher.forceActiveFocus() })
  }

  Process {
    id: listProc
    stdout: StdioCollector {
      waitForEnd: true
      onStreamFinished: root.parseState(text)
    }
  }

  Process {
    id: actionProc
    property string okMessage: ""
    stdout: StdioCollector {
      waitForEnd: true
      onStreamFinished: root.parseState(text)
    }
    onExited: function(code) {
      root.busy = false
      if (code === 0 && actionProc.okMessage !== "" && root.errorText === "")
        root.statusText = actionProc.okMessage
      else if (code !== 0 && root.errorText === "")
        root.errorText = "screen-controller exited " + code
    }
  }

  Timer {
    interval: Math.max(1000, root.refreshMs)
    running: true
    repeat: true
    triggeredOnStart: true
    onTriggered: if (!root.busy) root.refresh()
  }

  IpcHandler {
    target: root.ipcTarget
    function open(): void { root.open() }
    function close(): void { root.close() }
    function toggle(): void { root.toggle() }
    function refresh(): string { root.refresh(); return "ok" }
  }

  BarIconButton {
    id: button
    anchors.fill: parent
    bar: root.bar
    text: root.layoutIcon
    tooltipText: root.matchedId !== ""
      ? "Layout: " + root.matchedId
      : (root.errorText !== "" ? root.errorText : "Screen layouts")
    onPressed: function(buttonCode) {
      if (buttonCode === Qt.RightButton) root.openTui()
      else root.toggle()
    }
  }

  KeyboardPanel {
    id: panel
    anchorItem: button
    owner: root
    bar: root.bar
    open: root.opened
    focusTarget: keyCatcher
    contentWidth: panel.fittedContentWidth(Style.space(400))
    contentHeight: panel.fittedContentHeight(column.implicitHeight, Style.space(560))

    PanelKeyCatcher {
      id: keyCatcher
      anchors.fill: parent
      blocked: saveIdField.activeFocus || saveLabelField.activeFocus
      onMoveRequested: function(dx, dy) {
        if (root.mode !== "list") return
        if (!root.cursorActive) { root.cursorActive = true; return }
        if (dy !== 0) root.moveCursor(dy)
      }
      onActivateRequested: if (root.mode === "list" && root.cursorActive) root.applySelected()
      onCloseRequested: {
        if (root.mode !== "list") { root.mode = "list"; return }
        root.close()
      }
      onTabRequested: function(direction) { root.switchPanel(direction) }
      onTextKey: function(t) {
        if (t === "r" || t === "R") root.refresh()
        else if (t === "t" || t === "T") root.openTui()
        else if (t === "s" || t === "S") {
          root.mode = "save"
          root.saveId = ""
          root.saveLabel = ""
        }
        else if (t === "e" || t === "E") {
          root.mode = "relabel"
          root.saveLabel = root.selected ? root.selected.label : ""
        }
        else if (t === "d" || t === "D") root.askDelete()
      }

      Column {
        id: column
        width: parent.width
        spacing: Style.space(12)

        Item {
          width: parent.width
          implicitHeight: Math.max(heroIcon.implicitHeight, heroLabels.implicitHeight)

          Text {
            id: heroIcon
            text: root.layoutIcon
            color: root.bar.foreground
            font.family: root.bar.fontFamily
            font.pixelSize: Style.font.display
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
          }

          Column {
            id: heroLabels
            anchors.left: heroIcon.right
            anchors.leftMargin: Style.space(14)
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            spacing: Style.space(2)

            Text {
              text: "Layouts"
              color: root.bar.foreground
              font.family: root.bar.fontFamily
              font.pixelSize: Style.font.title
              font.bold: true
            }
            Text {
              text: (root.matchedId !== "" ? root.matchedId : "No matching profile")
                + (root.hyprland !== "" ? " · " + root.hyprland : "")
              color: Qt.darker(root.bar.foreground, 1.4)
              font.family: root.bar.fontFamily
              font.pixelSize: Style.font.caption
              elide: Text.ElideRight
              width: parent.width
            }
          }
        }

        Column {
          width: parent.width
          spacing: Style.space(4)
          visible: root.mode === "list"

          Repeater {
            model: root.profiles
            delegate: Button {
              required property var modelData
              required property int index
              width: parent.width
              text: (modelData.matched ? "●  " : "   ") + modelData.label
              selected: root.cursorActive && root.selectedIndex === index
              active: modelData.matched
              foreground: root.bar.foreground
              fontFamily: root.bar.fontFamily
              onClicked: {
                root.selectedIndex = index
                root.cursorActive = true
                root.applySelected()
              }
              onHovered: function(isHot) {
                if (!isHot) return
                root.selectedIndex = index
                root.cursorActive = true
              }
            }
          }
        }

        Column {
          width: parent.width
          spacing: Style.space(8)
          visible: root.mode === "save" || root.mode === "relabel"

          Text {
            text: root.mode === "save" ? "Save current layout" : "Edit label"
            color: root.bar.foreground
            font.family: root.bar.fontFamily
            font.pixelSize: Style.font.body
            font.bold: true
          }
          TextField {
            id: saveIdField
            width: parent.width
            visible: root.mode === "save"
            placeholderText: "id (e.g. dual_sdr)"
            text: root.saveId
            foreground: root.bar.foreground
            onTextChanged: root.saveId = text
          }
          TextField {
            id: saveLabelField
            width: parent.width
            placeholderText: "Label"
            text: root.saveLabel
            foreground: root.bar.foreground
            onTextChanged: root.saveLabel = text
          }
          Row {
            spacing: Style.space(8)
            Button {
              text: root.mode === "save" ? "Save" : "Update label"
              foreground: root.bar.foreground
              onClicked: root.mode === "save" ? root.saveCurrent() : root.relabelSelected()
            }
            Button {
              text: "Cancel"
              foreground: root.bar.foreground
              onClicked: root.mode = "list"
            }
          }
        }

        Column {
          width: parent.width
          spacing: Style.space(8)
          visible: root.mode === "confirmDelete"

          Text {
            text: "Delete " + root.pendingDeleteId + "?"
            color: root.bar.foreground
            font.family: root.bar.fontFamily
            font.pixelSize: Style.font.body
            wrapMode: Text.WordWrap
            width: parent.width
          }
          Row {
            spacing: Style.space(8)
            Button {
              text: "Delete"
              foreground: root.bar.foreground
              onClicked: root.confirmDelete()
            }
            Button {
              text: "Cancel"
              foreground: root.bar.foreground
              onClicked: { root.pendingDeleteId = ""; root.mode = "list" }
            }
          }
        }

        Text {
          width: parent.width
          visible: root.errorText !== "" || root.statusText !== ""
          text: root.errorText !== "" ? root.errorText : root.statusText
          color: root.errorText !== "" ? (root.bar.urgent || root.bar.foreground) : Qt.darker(root.bar.foreground, 1.45)
          font.family: root.bar.fontFamily
          font.pixelSize: Style.font.caption
          wrapMode: Text.WordWrap
        }

        Row {
          width: parent.width
          spacing: Style.space(8)
          visible: root.mode === "list"

          Button {
            text: "Save current"
            foreground: root.bar.foreground
            onClicked: { root.mode = "save"; root.saveId = ""; root.saveLabel = "" }
          }
          Button {
            text: "Edit label"
            enabled: root.selected !== null
            foreground: root.bar.foreground
            onClicked: { root.mode = "relabel"; root.saveLabel = root.selected ? root.selected.label : "" }
          }
          Button {
            text: "Delete"
            enabled: root.selected !== null && root.profiles.length > 1
            foreground: root.bar.foreground
            onClicked: root.askDelete()
          }
          Button {
            text: "TUI"
            foreground: root.bar.foreground
            onClicked: root.openTui()
          }
        }
      }
    }
  }
}
