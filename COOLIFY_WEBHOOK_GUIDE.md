# 🔧 Coolify 自动部署 Webhook 配置指南

**目标**: 解决 forked 仓库或其他项目在 `git push` 后无法自动触发 Coolify 部署的问题。

---

## 📖 核心原理：Webhook 是什么？

Webhook 就像一个“通知系统”。你希望在 `git push` 代码到 GitHub 后，GitHub 能自动通知 Coolify 来部署新版本。这个通知就是通过 Webhook 发送的。

对于 forked 仓库，Coolify 通常没有权限自动帮你设置这个通知，所以我们需要手动配置一次。

---

## 📋 操作步骤

### **第 1 步：在 Coolify 中找到你的 Webhook URL**

1.  进入你在 Coolify 中部署的应用 (无论是前端还是后端，它们通常使用相同的 Webhook)。
2.  在应用的设置页面，找到 **"Webhooks"** 或者 **"Deploy Webhook"** 相关的部分。
3.  你会看到一个唯一的 URL，类似 `https://app.coolify.io/api/v1/webhooks/deploy/github/...`。
4.  **复制这个 URL**。

### **第 2 步：在 GitHub 仓库中添加 Webhook**

1.  打开你那个 forked 的 GitHub 仓库页面。
2.  点击右上角的 **"Settings"**。
3.  在左侧菜单中，点击 **"Webhooks"**。
4.  点击右上角的 **"Add webhook"** 按钮。
5.  现在，配置这个新的 Webhook：
    *   **Payload URL**: 粘贴你刚刚从 Coolify 复制的 URL。
    *   **Content type**: 选择 `application/json`。
    *   **Secret**: 保持空白即可，除非 Coolify 特别提供了 Secret。
    *   **Which events would you like to trigger this webhook?**: 选择 **"Just the `push` event."** (这是默认选项，通常不用改)。
    *   **Active**: 确保这个复选框是勾选状态。
6.  点击页面底部的 **"Add webhook"** 按钮。

### **第 3 步：测试 Webhook**

1.  添加成功后，你会回到 Webhooks 列表页面。你应该能看到刚刚添加的那条记录。
2.  现在，对你的项目做一个小的代码修改，然后执行 `git push`。
3.  回到 GitHub 的 Webhook 设置页面，刷新一下，然后点击你创建的那个 Webhook 旁边的 **"Edit"**。
4.  切换到 **"Recent Deliveries"** (最近交付) 标签页。
5.  你应该能看到一个刚刚发生的交付记录。如果它前面是一个**绿色的小勾 ✅**，并且响应码是 `200`，那就代表 GitHub 已经成功通知了 Coolify！
6.  此时，回到你的 Coolify 面板，你应该能看到应用已经开始自动构建部署了。

---

## 🔍 故障排查

-   **收到 `401 Unauthorized` 错误?**
    -   这是最常见的错误，意味着你的请求没有被 Coolify 认证。
    -   **解决方案**: 你需要在 GitHub Webhook 设置中添加一个 `Secret`。
        1.  回到 Coolify 应用的 **Webhooks** 设置页面。
        2.  找到一个叫做 **"Deploy Token"** 或者 **"Secret"** 的东西。它是一长串随机字符。如果没有，可能会有一个 "Regenerate" (重新生成) 按钮。
        3.  **复制**这个 Token。
        4.  回到 GitHub 的 Webhook 设置页面，找到 **"Secret"** 输入框。
        5.  将你复制的 Token **粘贴**进去。
        6.  保存更改，然后再次 `git push` 测试。

-   **收到红色的叉 ❌ 但不是 401?** 
    -   如果错误不是 `401`，点击交付记录查看具体错误。最常见的原因是 Coolify 的 `Payload URL` 粘贴错了。

按照这个指南操作，你的 forked 仓库也能享受到丝滑的自动化部署体验了！