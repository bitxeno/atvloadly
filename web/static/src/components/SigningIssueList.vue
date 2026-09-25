<template>
  <div class="atv-signing-issues">
    <section
      v-for="group in groups"
      :key="group.severity"
      class="atv-signing-issue-group"
      :class="`atv-signing-issue-group--${group.severity}`"
    >
      <h5 class="atv-signing-issue-heading">
        {{ $t(`signing.severity.${group.severity}`) }}
      </h5>
      <ul>
        <li
          v-for="(issue, index) in group.issues"
          :key="`${issue.code}-${index}`"
          class="atv-signing-issue"
        >
          <span>{{ text(issue) }}</span>
          <span v-if="issue.message && issue.message !== text(issue)" class="atv-signing-issue-detail">
            {{ issue.message }}
          </span>
          <code v-if="issue.bundle" class="atv-signing-issue-bundle">{{ issue.bundle }}</code>
        </li>
      </ul>
    </section>
  </div>
</template>

<script>
import { groupIssuesBySeverity, issueText } from "@/utils/signing-report.mjs";

export default {
  name: "SigningIssueList",
  props: {
    issues: { type: Array, default: () => [] },
  },
  computed: {
    groups() {
      return groupIssuesBySeverity(this.issues);
    },
  },
  methods: {
    text(issue) {
      return issueText(issue, (key) => this.$t(key));
    },
  },
};
</script>

<style scoped>
.atv-signing-issues {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.atv-signing-issue-group {
  padding: 10px 12px;
  border: 1px solid var(--atv-border);
  border-left-width: 4px;
  border-radius: 12px;
  background: var(--atv-surface-alt);
}

.atv-signing-issue-group--error {
  border-color: var(--atv-danger);
  background: var(--atv-danger-soft);
}

.atv-signing-issue-group--warning {
  border-color: var(--atv-warning-border);
  background: var(--atv-warning-bg);
}

.atv-signing-issue-heading {
  margin: 0 0 6px;
  font-size: .78rem;
  font-weight: 760;
  letter-spacing: .02em;
  color: var(--atv-muted);
}

.atv-signing-issue-group--error .atv-signing-issue-heading {
  color: var(--atv-danger);
}

.atv-signing-issue-group--warning .atv-signing-issue-heading {
  color: var(--atv-warning-text);
}

.atv-signing-issue-group ul {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.atv-signing-issue {
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-size: .86rem;
  line-height: 1.4;
  color: var(--atv-ink);
  overflow-wrap: anywhere;
}

.atv-signing-issue-detail,
.atv-signing-issue-bundle {
  font-size: .74rem;
  color: var(--atv-muted);
}
</style>
