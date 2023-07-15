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
    && wget https://github.com/bitxeno/usbmuxd2/releases/download/v0.0.2/usbmuxd2-ubuntu-${PKG_ARCH}.tar.gz \
    && tar zxf usbmuxd2-ubuntu-${PKG_ARCH}.tar.gz \
    && dpkg -i --force-architecture ./libusb_1.0.26-1_${PKG_ARCH}.deb \
    && dpkg -i --force-architecture ./libgeneral_1.0.0-1_${PKG_ARCH}.deb \
    && dpkg -i --force-architecture ./libplist_2.3.0-1_${PKG_ARCH}.deb \
    && dpkg -i --force-architecture ./libimobiledevice-glue_1.0.0-1_${PKG_ARCH}.deb \
    && dpkg -i --force-architecture ./libusbmuxd_2.3.0-1_${PKG_ARCH}.deb \
    && dpkg -i --force-architecture ./libimobiledevice_1.3.1-1_${PKG_ARCH}.deb \
    && dpkg -i --force-architecture ./usbmuxd2_1.0.0-1_${PKG_ARCH}.deb

# 安装anisette-server，用于模拟本机为MacBook
RUN case ${TARGETARCH} in \
         "amd64")  PKG_ARCH=x86_64  ;; \
         "arm64")  PKG_ARCH=aarch64  ;; \
    esac \
    && cd /tmp \
    && wget https://github.com/Dadoum/Provision/releases/download/2.1.0/anisette-server-${PKG_ARCH} \
    && mv anisette-server-${PKG_ARCH} /usr/bin/anisette-server \
    && chmod +x /usr/bin/anisette-server

# 安装AltStore
RUN case ${TARGETARCH} in \
         "amd64")  PKG_ARCH=x86_64  ;; \
         "arm64")  PKG_ARCH=aarch64  ;; \
    esac \
    && cd /tmp \
    && wget https://github.com/NyaMisty/AltServer-Linux/releases/download/v0.0.5/AltServer-${PKG_ARCH} \
    && mv AltServer-${PKG_ARCH} /usr/bin/AltServer \
    && chmod +x /usr/bin/AltServer

# 安装tzdata支持更新时区
RUN DEBIAN_FRONTEND=noninteractive TZ=Asia/Shanghai apt-get -y install tzdata

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
RUN printf '#!/bin/sh \n\n\

mkdir -p /data/lockdown \n\
mkdir -p /data/AltServer \n\

if [ ! -f "/data/config.yaml" ]; then  \n\
    cp /doc/config.yaml /data/config.yaml \n\
fi  \n\

nohup /usr/sbin/usbmuxd & \n\
nohup /usr/bin/anisette-server --adi-path /data/Provision &  \n\

/usr/bin/%s server -p ${SERVICE_PORT:-80} -c /data/config.yaml  \n\
\n\
' ${APP_NAME} >> /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
# docker 启动不了，需要进入 docker 测试时使用本命令
# docker run -it --entrypoint /bin/sh [docker_image]

EXPOSE 80
VOLUME /data