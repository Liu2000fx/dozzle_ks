<template>
  <scrollable-view :scrollable="scrollable" v-if="container">
    <template #header v-if="showTitle">
      <div class="mr-0 columns is-mobile is-vcentered is-marginless has-boxshadow">
        <div class="column is-clipped is-paddingless">
          <container-title @close="$emit('close')" />
        </div>
        <div class="column is-narrow is-paddingless">
          <button
            :class="{ pressed: activeButton === 'button1' }"
            @click="searchInfo('button1')"
            class="is-hidden-mobile"
            style="color: #00b5ad; justify-content: flex-end"
          >
            <carbon:circle-solid class="is-small" style="color: #00b5ad" />
            {{ calenum[0] }}
          </button>
          <button
            :class="{ pressed: activeButton === 'button2' }"
            @click="searchWarn('button2')"
            class="is-hidden-mobile"
            style="color: #ff9800; justify-content: flex-end"
          >
            <carbon:circle-solid class="is-small" style="color: #ff9800" />
            {{ calenum[1] }}
          </button>
          <button
            :class="{ pressed: activeButton === 'button3' }"
            @click="searchError('button3')"
            class="is-hidden-mobile"
            style="color: #f44336; justify-content: flex-end"
          >
            <carbon:circle-solid class="is-small" style="color: #f44336" />
            {{ calenum[2] }}
          </button>
        </div>
        <div class="column is-narrow is-paddingless">
          <container-stat />
        </div>

        <div class="mr-2 column is-narrow is-paddingless is-hidden-mobile">
          <log-actions-toolbar @clear="onClearClicked()" />
        </div>
        <div class="mr-2 column is-narrow is-paddingless" v-if="closable">
          <button class="delete is-medium" @click="close()"></button>
        </div>
      </div>
    </template>
    <template #default="{ setLoading }">
      <log-viewer-with-source ref="viewer" @loading-more="setLoading($event)" @sendData="receiveDataFromChild" />
    </template>
  </scrollable-view>
</template>

<script lang="ts" setup>
import LogViewerWithSource from "./LogViewerWithSource.vue";

const {
  id,
  showTitle = false,
  scrollable = false,
  closable = false,
} = defineProps<{
  id: string;
  showTitle?: boolean;
  scrollable?: boolean;
  closable?: boolean;
}>();

const close = defineEmit();
let infonum2 = ref(0);
let calenum = ref<number[]>([]);
function receiveDataFromChild(data: number[]) {
  calenum.value = data;
}
const store = useContainerStore();

const container = store.currentContainer($$(id));
const config = reactive({ stdout: true, stderr: true });

provide("container", container);
provide("stream-config", config);
provide("info-num", infonum2);
const sendDataOK = defineEmit<[value: string]>();
let infostr = "";
let warnstr = "";
let errorstr = "";

const viewer = ref<InstanceType<typeof LogViewerWithSource>>();
const activeButton = ref("");

onKeyStroke("f", (e) => {
  if (e.ctrlKey || e.metaKey) {
    activeButton.value = "";
    infostr = "";
    warnstr = "";
    errorstr = "";
    sendDataOK(infostr);
    e.preventDefault();
  }
});

function onClearClicked() {
  viewer.value?.clear();
}
function searchInfo(str: string) {
  console.log("搜索info");
  activeButton.value = activeButton.value === str ? "" : str;
  if (infostr === "") {
    infostr = "info";
    warnstr = "";
    errorstr = "";
  } else {
    infostr = "";
    warnstr = "";
    errorstr = "";
  }
  sendDataOK(infostr);
}
function searchWarn(str: string) {
  console.log("搜索warn");
  activeButton.value = activeButton.value === str ? "" : str;
  if (warnstr === "") {
    warnstr = "warn";
    infostr = "";
    errorstr = "";
  } else {
    infostr = "";
    warnstr = "";
    errorstr = "";
  }
  sendDataOK(warnstr);
}
function searchError(str: string) {
  console.log("搜索error");
  activeButton.value = activeButton.value === str ? "" : str;
  if (errorstr === "") {
    errorstr = "error";
    infostr = "";
    warnstr = "";
  } else {
    infostr = "";
    warnstr = "";
    errorstr = "";
  }
  sendDataOK(errorstr);
}
</script>
<style lang="scss" scoped>
button.delete {
  background-color: var(--scheme-main-ter);
  opacity: 0.6;

  &:after,
  &:before {
    background-color: var(--text-color);
  }

  &:hover {
    opacity: 1;
  }
}

button.pressed {
  background-color: #ccc;
  /* 设置按钮按下时的背景颜色 */
  color: #fff;
  /* 设置按钮按下时的文字颜色 */
}
</style>
