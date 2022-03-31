FROM digineode/texd:base

COPY texd /bin/

WORKDIR /texd

EXPOSE 2201

ENTRYPOINT ["/bin/texd", "--job-directory", "/texd"]
