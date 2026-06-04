<template>
  <div class="not-found">
    <div class="content">
      <h1>404</h1>
      <p>页面不存在</p>
      <el-button type="primary" @click="$router.push('/')">返回首页</el-button>
    </div>
  </div>
</template>

<style lang="scss" scoped>
.not-found {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  .content {
    text-align: center;
    h1 { font-size: 120px; font-weight: 700; color: #409eff; margin: 0; line-height: 1; }
    p { font-size: 18px; color: #909399; margin: 16px 0 24px; }
  }
}
</style>
