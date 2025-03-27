FROM python:3.11-slim-buster

RUN apt update && apt install -y \
    build-essential \
    curl \
    libcfitsio-dev \
    libgsl-dev \
    libjpeg-dev \
    libpng-dev \
    libtiff-dev \
    libx11-dev \
    libxext-dev \
    libxft-dev \
    libxmu-dev \
    libxpm-dev \
    libxt-dev \
    wget \
    && apt-get clean

RUN curl -O http://montage.ipac.caltech.edu/download/Montage_v6.0.tar.gz && \
    tar xvfz Montage_v6.0.tar.gz && \
    rm Montage_v6.0.tar.gz && \
    cd Montage && \
    make

ENV PATH="/Montage/bin:${PATH}"

RUN python -m pip install --upgrade pip && \
    python -m pip install \
        'astropy' \
        'Flask-Caching>1.8'     \
        'Flask<2.3,>1.1'        \
        'GitPython>1.0'         \
        'PyYAML>5.3'            \
        'pamela<1.1.0,>=1.0'    \
        'pegasus-wms.api'       \
        'pegasus-wms.common'    \
        'pegasus-wms.dax'       \
        'pegasus-wms.worker'    \
        'pika>=1.1.0'           \
        'requests>2.23'         \
        'sqlalchemy<1.4,>1.3'   \
        'debugpy'

