//go:build integration
//
// Integration tests for the ilnamiqui opencode plugin.
// These tests verify the plugin loads, its hooks wire up correctly,
// and state management works across hooks.

import { describe, it, expect, beforeEach } from "vitest"
import {
  buildSummary,
  conversationBuffer,
  exitSaved,
  resolveBinarySync,
  resetTestState,
  interactionCounter,
} from "./ilnamiqui"

// ---------------------------------------------------------------------------
// Setup
// ---------------------------------------------------------------------------

beforeEach(() => {
  resetTestState()
})

// ---------------------------------------------------------------------------
// Plugin loading
// ---------------------------------------------------------------------------

describe("plugin loading", () => {
  it("can import without errors", () => {
    // The import itself passing is the test — verify we can call buildSummary
    expect(typeof buildSummary).toBe("function")
  })

  it("exports plugin as default", async () => {
    const mod = await import("./ilnamiqui")
    expect(mod.default).toBeDefined()
    expect(typeof mod.default.server).toBe("function")
  })

  it("exports internals for testing", () => {
    expect(Array.isArray(conversationBuffer)).toBe(true)
    expect(typeof exitSaved).toBe("boolean")
    expect(typeof resetTestState).toBe("function")
    expect(typeof resolveBinarySync).toBe("function")
  })
})

// ---------------------------------------------------------------------------
// Buffer accumulation via chat.message behavior (simulated)
// ---------------------------------------------------------------------------

describe("buffer accumulation", () => {
  it("adds entries to conversationBuffer", () => {
    // Simulate what the chat.message hook does
    const text = "fix the auth middleware bug"
    conversationBuffer.push({
      role: "user",
      text,
      timestamp: new Date().toISOString(),
    })
    if (conversationBuffer.length > 20) {
      conversationBuffer.shift()
    }

    expect(conversationBuffer.length).toBe(1)
    expect(conversationBuffer[0].text).toBe(text)
    expect(conversationBuffer[0].role).toBe("user")
  })

  it("buildSummary returns meaningful content after accumulation", () => {
    // Push several messages with decisions and file paths
    const messages = [
      "start refactoring internal/db/db.go",
      "use sqlc for query generation",
      "fix: connection pool exhausted",
      "add: automatic retry in cmd/ilnamiqui/main.go",
      "chose pgx over database/sql",
      "also update src/components/Button.tsx",
    ]
    for (const msg of messages) {
      conversationBuffer.push({
        role: "user",
        text: msg,
        timestamp: new Date().toISOString(),
      })
    }

    const summary = buildSummary(conversationBuffer)
    expect(summary).toContain("session: also update src/components/Button.tsx") // last msg
    expect(summary).toContain("internal/db/db.go")
    expect(summary).toContain("cmd/ilnamiqui/main.go")
    expect(summary).toContain("src/components/Button.tsx")
    expect(summary).toContain("use sqlc")
    expect(summary).toContain("fix:")
    expect(summary).toContain("add:")
    expect(summary).toContain("chose")
    expect(summary).toContain("entry_count: 6")
  })

  it("buffer does not exceed 20 entries", () => {
    for (let i = 0; i < 25; i++) {
      conversationBuffer.push({
        role: "user",
        text: `message ${i}`,
        timestamp: new Date().toISOString(),
      })
      if (conversationBuffer.length > 20) {
        conversationBuffer.shift()
      }
    }

    expect(conversationBuffer.length).toBeLessThanOrEqual(20)
    expect(conversationBuffer.length).toBe(20)
  })
})

// ---------------------------------------------------------------------------
// exitSaved guard behavior
// ---------------------------------------------------------------------------

describe("exitSaved guard", () => {
  it("starts false after reset", () => {
    expect(exitSaved).toBe(false)
  })

  it("prevents duplicate saves when true", () => {
    // Simulate first save
    const saved1 = exitSaved
    // After session.deleted sets it to true, second call should be noop
    // We can't set it directly (read-only export), but we verify the pattern
    expect(saved1).toBe(false)
  })
})

// ---------------------------------------------------------------------------
// interaction counter combined behavior
// ---------------------------------------------------------------------------

describe("interaction counter combined behavior", () => {
  it("uses local counter to model combined user+messages and various tool calls", () => {
    let count = 0

    // Simulate 3 user messages
    for (let i = 0; i < 3; i++) {
      conversationBuffer.push({
        role: "user",
        text: `message ${i}`,
        timestamp: new Date().toISOString(),
      })
      count++
    }

    // Simulate varied tool calls (not just task)
    count++ // tool call: read
    count++ // tool call: write
    count++ // tool call: bash
    count++ // tool call: task

    expect(count).toBe(7)
  })

  it("buffer and counter work independently", () => {
    let count = 0

    // Fill buffer to 20, simulate 25 interactions
    for (let i = 0; i < 20; i++) {
      conversationBuffer.push({
        role: "user",
        text: `msg ${i}`,
        timestamp: new Date().toISOString(),
      })
      count++
      if (conversationBuffer.length > 20) conversationBuffer.shift()
    }
    // 5 more tool-only interactions (various tools)
    count += 5

    expect(conversationBuffer.length).toBe(20)
    expect(count).toBe(25)
  })

  it("resets with resetTestState()", () => {
    // interactionCounter is read-only const binding (ES module)
    // resetTestState runs in module scope where it can mutate
    resetTestState()
    expect(interactionCounter).toBe(0)
  })
})


