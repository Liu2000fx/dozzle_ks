<template>
  <infinite-loader :onLoadMore="fetchMore" :enabled="messages.length > 100"></infinite-loader>
  <slot :messages="messages"></slot>
</template>

<script lang="ts" setup>
import { Container } from "@/models/Container";
import { type ComputedRef } from "vue";

const loadingMore = defineEmit<[value: boolean]>();
const sendData = defineEmit<[value: number[]]>();

const container = inject("container") as ComputedRef<Container>;
const config = inject("stream-config") as { stdout: boolean; stderr: boolean };

const { messages, calenum, loadOlderLogs } = useLogStream(container, config);

watch(calenum, (newValue) => {
  sendData(calenum.value);
});

const beforeLoading = () => loadingMore(true);
const afterLoading = () => loadingMore(false);

defineExpose({
  clear: () => (messages.value = []),
});

const fetchMore = () => loadOlderLogs({ beforeLoading, afterLoading });
</script>
