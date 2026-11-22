import type { Register } from 'claude-code'

export const register: Register = on => {
  on('session.start', async ($, e, next) => {
    await $.command.register({
      name: 'send-message',
      description: 'Send a message to another session',
      argumentHint: '<session id> <text>',
    })
    return next(e)
  })

  on('command.run', { command: 'send-message' }, async ($, e) => {
    const m = e.args.trim().match(/^(\S+)\s+([\s\S]+)$/)
    if (!m) return { text: 'usage: /send-message <session id> <text>', exitCode: 2 }
    const [, sessionId, text] = m
    const sent = await $.session.send({ to: { sessionId }, text })
    return sent.isDelivered
      ? { text: `sent to ${sessionId}`, isTextDisplayOnly: true, exitCode: 0 }
      : { text: `not sent: ${sent.reason}`, isTextDisplayOnly: true, exitCode: 1 }
  })
}
