FROM texlive/texlive:latest

COPY texd /bin/

WORKDIR /texd

EXPOSE 2201

ENTRYPOINT ["/bin/texd", "--job-directory", "/texd"]
