import axios from "axios";
import { toast } from "vue3-toastify";

// create an axios instance
const service = axios.create({
  baseURL: "/", // url = base url + request url
  // withCredentials: true, // send cookies when cross-domain requests
  timeout: 5000, // request timeout
});

// request interceptor
service.interceptors.request.use(
  (config) => {
    // do something before request is sent
    return config;
  },
  (error) => {
    // do something with request error
    console.log(error); // for debug
    return Promise.reject(error);
  }
);

// response interceptor
service.interceptors.response.use(
  /**
   * If you want to get http information such as headers or status
   * Please return  response => response
   */

  /**
   * Determine the request status by custom code
   * Here is just an example
   * You can also judge the status by HTTP Status Code
   */
  (response) => {
    const res = response.data;

    if (response.config.responseType === 'blob') {
      return response;
    }

    // if the custom code is not 200, it is judged as an error.
    if (res.code !== 200) {
      const msg = res.msg ? `${res.msg} (code: ${res.code})` : `Error (code: ${res.code})`;
      toast.error(msg, { autoClose: 5000 });
      return Promise.reject(new Error(msg));
    } else {
      return res;
    }
  },
  (error) => {
    console.log("err" + error); // for debug
    toast.error(error.message, { autoClose: 5000 });
    return Promise.reject(error);
  }
);

export default service;
