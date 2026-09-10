<template>
  <div id="app">
    <div class="chat-container">
      <div class="chat-history" ref="chatHistory">
        <div v-for="(message, index) in messages" :key="index" class="message" :class="message.sender">
          <div class="avatar">{{ message.sender === 'user' ? '你' : 'AI' }}</div>
          <div class="text" v-html="formatMessage(message.text)"></div>
        </div>
      </div>
      <div class="chat-input">
        <input
          v-model="userInput"
          @keyup.enter="sendMessage"
          placeholder="请输入你的问题..."
          :disabled="isLoading"
        />
        <button @click="sendMessage" :disabled="isLoading">
          {{ isLoading ? '思考中...' : '发送' }}
        </button>
      </div>
    </div>
  </div>
</template>

<script>
import axios from 'axios';
import { marked } from 'marked';

export default {
  name: 'App',
  data() {
    return {
      messages: [
        { sender: 'bot', text: '你好！我是你的个人智能助手。有什么可以帮你的吗？' }
      ],
      userInput: '',
      isLoading: false,
    };
  },
  methods: {
    async sendMessage() {
      if (this.userInput.trim() === '' || this.isLoading) return;

      const userMessage = this.userInput;
      this.messages.push({ sender: 'user', text: userMessage });
      this.userInput = '';
      this.isLoading = true;
      this.scrollToBottom();

      try {
        const response = await axios.post('/api/ask', {
          query: userMessage,
        });

        let botResponse = response.data.answer;
        if (response.data.sources && response.data.sources.length > 0) {
          botResponse += '\n\n**参考来源:**\n';
          response.data.sources.forEach((source, i) => {
            botResponse += `${i + 1}. \`\`\`\n${source}\n\`\`\`\n`;
          });
        }
        this.messages.push({ sender: 'bot', text: botResponse });

      } catch (error) {
        console.error('发送消息时出错:', error);
        this.messages.push({ sender: 'bot', text: '抱歉，我遇到了一个错误，请稍后再试。' });
      } finally {
        this.isLoading = false;
        this.scrollToBottom();
      }
    },
    scrollToBottom() {
      this.$nextTick(() => {
        const container = this.$refs.chatHistory;
        if (container) {
          container.scrollTop = container.scrollHeight;
        }
      });
    },
    formatMessage(text) {
      return marked(text);
    }
  }
};
</script>

<style>
  #app { display: flex; justify-content: center; align-items: center; height: 100vh; background-color: #f0f2f5; font-family: sans-serif; }
  .chat-container { width: 80%; max-width: 800px; height: 90vh; border: 1px solid #ccc; border-radius: 8px; display: flex; flex-direction: column; background-color: white; }
  .chat-history { flex: 1; padding: 20px; overflow-y: auto; border-bottom: 1px solid #ccc; }
  .message { display: flex; margin-bottom: 15px; }
  .message.user { flex-direction: row-reverse; }
  .avatar { width: 40px; height: 40px; border-radius: 50%; background-color: #007bff; color: white; display: flex; justify-content: center; align-items: center; font-weight: bold; margin: 0 10px; }
  .message.bot .avatar { background-color: #28a745; }
  .text { padding: 10px; border-radius: 8px; background-color: #f1f0f0; max-width: 70%; }
  .message.user .text { background-color: #007bff; color: white; }
  .chat-input { display: flex; padding: 10px; }
  input { flex: 1; border: 1px solid #ccc; border-radius: 20px; padding: 10px 15px; font-size: 16px; }
  button { border: none; background-color: #007bff; color: white; border-radius: 20px; padding: 10px 20px; margin-left: 10px; cursor: pointer; }
  button:disabled { background-color: #a0a0a0; }
  pre { background-color: #eee; padding: 10px; border-radius: 5px; white-space: pre-wrap; word-wrap: break-word; }
  code { font-family: monospace; }
</style>
