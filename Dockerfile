FROM ubuntu:22.04
ARG APP_NAME
ARG VERSION
ARG BUILDDATE
ARG COMMIT
ARG TARGETPLATFORM
ARG TARGETOS
ARG TARGETARCH
RUN echo "I'm building for $TARGETPLATFORM"

# 安装依赖
RUN apt-get update && apt-get -y install \
    wget libavahi-compat-libdnssd-dev curl

RUN case ${TARGETARCH} in \
         "amd64")  PKG_ARCH=x86_64  ;; \
         "arm64")  PKG_ARCH=aarch64  ;; \
    esac \
    && cd /tmp \
    && wget https://github.com/bitxeno/usbmuxd2/releases/download/v0.0.4/usbmuxd2-ubuntu-${PKG_ARCH}.tar.gz \
    && tar zxf usbmuxd2-ubuntu-${PKG_ARCH}.tar.gz \
    && dpkg -i ./libusb_1.0.26-1_${PKG_ARCH}.deb \
    && dpkg -i ./libgeneral_1.0.0-1_${PKG_ARCH}.deb \
    && dpkg -i ./libplist_2.6.0-1_${PKG_ARCH}.deb \
    && dpkg -i ./libtatsu_1.0.3-1_${PKG_ARCH}.deb \
    && dpkg -i ./libimobiledevice-glue_1.3.0-1_${PKG_ARCH}.deb \
    && dpkg -i ./libusbmuxd_2.3.0-1_${PKG_ARCH}.deb \
    && dpkg -i ./libimobiledevice_1.3.1-1_${PKG_ARCH}.deb \
    && dpkg -i ./usbmuxd2_1.0.0-1_${PKG_ARCH}.deb

# 安装Sideloader
RUN case ${TARGETARCH} in \
         "amd64")  PKG_ARCH=x86_64  ;; \
         "arm64")  PKG_ARCH=aarch64  ;; \
    esac \
    && cd /tmp \
    && wget https://github.com/bitxeno/Sideloader/releases/download/1.0-alpha.6/sideloader-cli-${PKG_ARCH}-linux-gnu.tar.gz \
    && tar zxf sideloader-cli-${PKG_ARCH}-linux-gnu.tar.gz \
    && mv sideloader-cli-${PKG_ARCH}-linux-gnu /usr/bin/sideloader \
    && chmod +x /usr/bin/sideloader

# 安装tzdata支持更新时区
RUN DEBIAN_FRONTEND=noninteractive apt-get -y install tzdata

# 清空apt缓存和临时数据，减小镜像大小
RUN apt-get clean
RUN cd /tmp && rm ./*.deb && rm ./*.tar.gz

# add 指令会自动解压文件
RUN mkdir -p /doc
COPY ./doc/config.yaml.example /doc/config.yaml
COPY ./build/${APP_NAME}-${TARGETOS}-${TARGETARCH} /usr/bin/${APP_NAME}
RUN chmod +x /usr/bin/${APP_NAME}

# lockdown记录移到到/data
RUN rm -rf /var/lib/lockdown && mkdir -p /data/lockdown && ln -s /data/lockdown /var/lib/lockdown



# 生成启动脚本
COPY ./doc/scripts/usbmuxd /etc/init.d/usbmuxd
RUN chmod +x /etc/init.d/usbmuxd
RUN printf '#!/bin/sh \n\n\

mkdir -p /data/lockdown \n\
mkdir -p /data/Sideloader \n\

if [ ! -f "/data/config.yaml" ]; then  \n\
    cp /doc/config.yaml /data/config.yaml \n\
fi  \n\

/etc/init.d/usbmuxd start \n\

/usr/bin/%s server -p ${SERVICE_PORT:-80} -c /data/config.yaml  \n\
\n\
' ${APP_NAME} >> /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
# docker 启动不了，需要进入 docker 测试时使用本命令
# docker run -it --entrypoint /bin/sh [docker_image]

EXPOSE 80
VOLUME /data