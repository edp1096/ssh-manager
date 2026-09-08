// Promise-based application dialogs. Callers await the choice before continuing.
const appDialogs = (() => {
    let tail = Promise.resolve(), sequence = 0
    function show(kind, message, options = {}) {
        const task = tail.then(() => new Promise(resolve => {
            const previous = document.activeElement
            const dialog = document.createElement('dialog'); dialog.className = 'app-message-dialog'; dialog.id = 'app-message-dialog'
            const title = document.createElement('h2'); title.id = `app-dialog-title-${++sequence}`
            title.textContent = options.title || (kind === 'alert' ? 'Notice' : kind === 'prompt' ? 'New folder' : 'Confirm action')
            dialog.setAttribute('aria-labelledby', title.id)
            const text = document.createElement('p'); text.className = 'app-dialog-message'; text.textContent = message
            const form = document.createElement('form')
            const actions = document.createElement('footer'); actions.className = 'modal-actions'
            let input, result = kind === 'prompt' ? null : false
            if (kind === 'prompt') {
                const label = document.createElement('label'); label.textContent = options.label || 'Name'
                input = document.createElement('input'); input.type = 'text'; input.required = true; input.autocomplete = 'off'; input.value = options.value || ''
                label.append(input); form.append(label)
            }
            if (kind !== 'alert') {
                const cancel = document.createElement('button'); cancel.type = 'button'; cancel.textContent = 'Cancel'; cancel.dataset.decision = 'cancel'
                cancel.title = 'Cancel (Esc)'
                cancel.addEventListener('click', () => dialog.close()); actions.append(cancel)
            }
            const accept = document.createElement('button'); accept.type = 'submit'; accept.dataset.decision = 'accept'
            accept.textContent = options.confirmText || (kind === 'alert' ? 'Close' : kind === 'prompt' ? 'Create folder' : 'Continue')
            if (kind === 'prompt') accept.title = `${accept.textContent} (Enter in text field)`
            else if (kind === 'alert') accept.title = `${accept.textContent} (Esc)`
            accept.className = options.danger ? 'critical' : 'ok'; actions.append(accept)
            form.append(actions)
            form.addEventListener('submit', event => { event.preventDefault(); result = input ? input.value : true; dialog.close() })
            dialog.append(title, text, form); document.body.append(dialog)
            dialog.addEventListener('close', () => {
                if (input) input.value = ''
                dialog.remove()
                if (previous?.isConnected && previous.getClientRects().length) previous.focus({ preventScroll: true })
                resolve(result)
            }, { once: true })
            dialog.showModal()
            if (input) { input.focus(); input.select() }
            else (actions.querySelector('[data-decision="cancel"]') || accept).focus()
        }))
        tail = task.catch(() => {})
        return task
    }
    return {
        alert: (message, options) => show('alert', message, options),
        confirm: (message, options) => show('confirm', message, options),
        prompt: (message, options) => show('prompt', message, options),
    }
})()

// Let IMEs own composition keys. Only a separate Enter submits a text field.
const modalComposing = new WeakSet()
document.addEventListener('compositionstart', event => { if (event.target.closest('dialog')) modalComposing.add(event.target) }, true)
document.addEventListener('compositionend', event => modalComposing.delete(event.target), true)
document.addEventListener('keydown', event => {
    const dialog = event.target.closest('dialog[open]')
    if (!dialog) return
    const composing = event.isComposing || event.keyCode === 229 || modalComposing.has(event.target)
    if (event.key === 'Enter' && composing) { event.preventDefault(); event.stopImmediatePropagation(); return }
    if (event.key === 'Enter' && event.target.matches('input:not([type="radio"]):not([type="checkbox"])')) {
        event.preventDefault(); event.stopImmediatePropagation()
        if (!event.repeat && event.target.form) {
            const form = event.target.form
            // Legacy method=dialog forms use the submit button's value to
            // distinguish Save/Unlock from Cancel in their close handlers.
            const submit = form.querySelector('button:not([type="button"]):not([type="reset"]), input[type="submit"]')
            if (!submit?.disabled) form.requestSubmit(submit || undefined)
        }
    }
    if (event.key === 'Escape' && !composing && dialog.id !== 'dialog-enter-password') {
        event.preventDefault(); event.stopImmediatePropagation()
        if (dialog.dispatchEvent(new Event('cancel', { cancelable: true }))) dialog.close('cancel')
    }
}, true)
