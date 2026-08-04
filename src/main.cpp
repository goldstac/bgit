#include <iostream>
#include <string>
#include <cstdlib>
#include <sstream>
int main(){
std::string input;
std::string commit_msg = "update";
std::string command = "";
std::cout << "Welcome To BGIT\n";
std::cout << "Options\n";
std::cout << "[1] Add,Commit,Push\n";
std::cout << "Enter Your Choice --> ";
std::getline(std::cin,input);
std::cout << input;
std::stringstream ss1;
if (input == "1"){
    ss1 << "git add . && git commit -m \"" << commit_msg << "\" && git push";
    command = ss1.str();
    std::system(command.c_str());
}
}