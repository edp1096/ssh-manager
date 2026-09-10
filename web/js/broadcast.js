const inputBroadcast = (() => {
    const tool = document.querySelector('#broadcast-tool')
    const indicator = document.querySelector('#broadcast-indicator')
    const dialog = document.createElement('dialog')
    dialog.id = 'broadcast-dialog'
    dialog.setAttribute('aria-labelledby', 'broadcast-title')
    dialog.innerHTML = `<h2 id="broadcast-title">Broadcast input</h2>
        <p>Choose one source and the connections that receive its input. Only app-opened SSH connections appear here.</p>
        <p class="broadcast-warning">Passwords and destructive commands are also sent. Turn off the terminal’s own input synchronization. Ctrl+] in a selected terminal stops broadcasting.</p>
        <div class="broadcast-list"><table><thead><tr><th>Source</th><th><label class="broadcast-select-all" title="Select all receivers"><input type="checkbox" data-select-all aria-label="Select all receivers">Receive</label></th><th>Connection</th></tr></thead><tbody></tbody></table></div>
        <p class="broadcast-status" role="status"></p>
        <footer class="modal-actions"><button type="button" data-close title="Close (Esc)">Close</button><button type="button" data-stop class="critical" title="Stop broadcast (Ctrl+])">Stop</button><button type="button" data-start class="ok">Start</button></footer>`
    document.body.append(dialog)
    const body = dialog.querySelector('tbody')
    const status = dialog.querySelector('.broadcast-status')
    const start = dialog.querySelector('[data-start]')
    const stop = dialog.querySelector('[data-stop]')
    const selectAll = dialog.querySelector('[data-select-all]')
    let state = { connections: [], source: '', targets: [], enabled: false }
    let signature = '', busy = false, polling = false, available = false, revision = 0
    dialog.querySelector('[data-close]').onclick = () => dialog.close()
    function draw(force = false) {
        indicator.textContent = available ? (state.enabled ? 'ON' : 'OFF') : '?'
        tool.classList.toggle('broadcast-on', available && state.enabled)
        tool.title = available && state.enabled ? 'Broadcast input ON — click to manage or stop' : 'Broadcast input'
        start.disabled = busy || !available || state.enabled || state.connections.length < 2
        stop.disabled = busy || (available && !state.enabled)
        if (!dialog.open) return
        const next = JSON.stringify(state)
        if (next !== signature || force) {
            const chosenSource = body.querySelector('input[type="radio"]:checked')?.value
            const chosenTargets = [...body.querySelectorAll('input[type="checkbox"]:checked')].map(e => e.value)
            body.replaceChildren()
            for (const connection of state.connections) {
                const row = document.createElement('tr')
                for (const kind of ['radio', 'checkbox']) {
                    const cell = document.createElement('td'), input = document.createElement('input')
                    input.type = kind; input.name = kind === 'radio' ? 'broadcast-source' : 'broadcast-target'
                    input.value = connection.id; input.disabled = busy || state.enabled
                    input.setAttribute('aria-label', `${kind === 'radio' ? 'Source' : 'Receive'}: ${connection.label} (${connection.id})`)
                    input.checked = kind === 'radio' ? connection.id === (state.enabled ? state.source : chosenSource)
                        : (state.enabled ? state.targets : chosenTargets).includes(connection.id)
                    cell.append(input); row.append(cell)
                }
                const label = document.createElement('td')
                label.textContent = `${connection.label} · ${connection.id.slice(0, 8)}`
                label.title = `${connection.label}\nConnection ID: ${connection.id}`
                row.append(label); body.append(row)
            }
            signature = next
        }
        for (const input of body.querySelectorAll('input')) input.disabled = busy || !available || state.enabled
        const source = body.querySelector('input[type="radio"]:checked')?.value
        for (const input of body.querySelectorAll('input[type="checkbox"]')) {
            if (input.value === source) { input.checked = false; input.disabled = true }
        }
        const receivers = [...body.querySelectorAll('input[type="checkbox"]')].filter(input => input.value !== source)
        const checked = receivers.filter(input => input.checked).length
        selectAll.disabled = busy || !available || state.enabled || !source || !receivers.length
        selectAll.checked = receivers.length > 0 && checked === receivers.length
        selectAll.indeterminate = checked > 0 && checked < receivers.length
        start.disabled ||= !source || !body.querySelector('input[type="checkbox"]:checked')
        status.textContent = !available ? 'Connection status unavailable. Use Ctrl+] in a selected terminal to stop.'
            : state.enabled ? `ON — ${state.targets.length} receiver(s). Closing this dialog does not stop broadcasting.`
            : state.reason || (state.connections.length < 2 ? 'Open at least two SSH connections. New connections are never selected automatically.' : 'OFF — select a source and receivers, then Start.')
        // Rebuilding rows or disabling the clicked Start/Stop button can drop
        // focus onto body, outside the modal's Escape/Tab handlers.
        if (document.activeElement === document.body || (dialog.contains(document.activeElement) && document.activeElement.disabled))
            (stop.disabled ? dialog.querySelector('[data-close]') : stop).focus()
    }
    body.addEventListener('change', () => draw())
    selectAll.addEventListener('change', () => {
        if (selectAll.disabled) return
        for (const input of body.querySelectorAll('input[type="checkbox"]:not(:disabled)')) input.checked = selectAll.checked
        draw()
    })
    async function refresh() {
        if (polling || busy) return
        polling = true
        const current = revision
        try {
            const response = await fetch('/session/broadcast', { cache: 'no-store' })
            if (!response.ok) throw new Error('Status unavailable')
            const update = await response.json()
            if (current === revision) { state = update; available = true }
        } catch { if (current === revision) available = false }
        finally { polling = false; draw() }
    }
    async function configure(source, targets) {
        if (busy) return
        busy = true; revision++; draw()
        try {
            const response = await fetch('/session/broadcast', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ source, targets }) })
            if (!response.ok) throw new Error(await response.text())
            state = await response.json(); available = true
        } catch (error) { available = false; await appDialogs.alert(error.message, { title: 'Broadcast input' }) }
        finally { busy = false; draw(true) }
    }
    start.onclick = () => configure(body.querySelector('input[type="radio"]:checked')?.value || '', [...body.querySelectorAll('input[type="checkbox"]:checked')].map(e => e.value))
    stop.onclick = () => configure('', [])
    setInterval(refresh, 1000)
    refresh()
    return { open() { if (!dialog.open) dialog.showModal(); draw(true); refresh() } }
})()
