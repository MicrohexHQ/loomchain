pragma solidity >=0.4.18;
contract TestEvent {
    event MyEvent(uint indexed number);

    function sendEvent(uint i) public   {
        emit MyEvent(i);
    }
}