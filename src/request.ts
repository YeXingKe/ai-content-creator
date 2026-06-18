import axios from 'axios'
import { message } from 'ant-design-vue'

// 创建 Axios 实例
const apiRequest = axios.create({
  baseURL: 'http://localhost:8567/api',
  timeout: 60000,
  withCredentials: true, // 必须！携带 Cookie
})

// 全局响应拦截器
apiRequest.interceptors.response.use(
  function (response: any) {
    const { data } = response
    // 未登录
    if (data.code === 40100) {
      if (
        !response.request.responseURL.includes('user/get/login') &&
        !window.location.pathname.includes('/user/login')
      ) {
        message.warning('请先登录')
        window.location.href = `/user/login?redirect=${window.location.href}`
      }
    }
    return response
  },
  function (error: any) {
    return Promise.reject(error)
  },
)

export default apiRequest
